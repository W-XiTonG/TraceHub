package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/paymentorder"
	"tracehub/internal/ent/rechargepackage"
	"tracehub/internal/pkg/config"

	"github.com/google/uuid"
)

var (
	ErrProviderDisabled = errors.New("payment provider disabled")
	ErrInvalidAmount    = errors.New("invalid payment amount")
	ErrOrderNotFound    = errors.New("order not found")
	ErrOrderNotPending  = errors.New("order not pending")
	ErrInsufficientBal  = errors.New("insufficient balance for refund")
)

// Service 支付主服务。
type Service struct {
	Client      *ent.Client
	AppConfig   *config.Config
	ConfigStore *ConfigStore
	providers   map[string]Provider
}

func NewService(client *ent.Client, appCfg *config.Config, store *ConfigStore) *Service {
	s := &Service{
		Client:      client,
		AppConfig:   appCfg,
		ConfigStore: store,
		providers: map[string]Provider{
			ProviderMock:    mockProvider{},
			ProviderAlipay:  alipayProvider{},
			ProviderWxpay:   wxpayProvider{},
			ProviderStripe:  stripeProvider{},
			ProviderEasyPay: easypayProvider{},
		},
	}
	return s
}

func (s *Service) Provider(name string) (Provider, bool) {
	p, ok := s.providers[name]
	return p, ok
}

func (s *Service) RuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	if s.ConfigStore == nil {
		return RuntimeConfig{MockEnabled: true, EnabledProviders: []string{ProviderMock}}, nil
	}
	return s.ConfigStore.Load(ctx)
}

func (s *Service) IsProviderEnabled(cfg RuntimeConfig, name string) bool {
	name = strings.TrimSpace(name)
	switch name {
	case ProviderMock:
		return cfg.MockEnabled
	case ProviderAlipay:
		return cfg.AlipayEnabled && cfg.AlipayAppID != "" && cfg.AlipayPrivateKey != ""
	case ProviderWxpay:
		return cfg.WxpayEnabled && cfg.WxpayMchID != "" && cfg.WxpayPrivateKey != ""
	case ProviderStripe:
		return cfg.StripeEnabled && cfg.StripeSecretKey != ""
	case ProviderEasyPay:
		return cfg.EasyPayEnabled && cfg.EasyPayPID != "" && cfg.EasyPayKey != ""
	default:
		return false
	}
}

type CreateOrderInput struct {
	UserID            int
	Provider          string
	PackageID         *int
	CustomAmountCents *int64
	Currency          string
	InvoiceRequested  bool
}

type CreateOrderOutput struct {
	OrderNo string
	Status  string
	PayURL  string
	QRCode  string
	ExpireAt time.Time
	AmountCents int64
	QuotaAmount int64
}

func (s *Service) CreateOrder(ctx context.Context, in CreateOrderInput) (*CreateOrderOutput, error) {
	cfg, err := s.RuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.OnlineEnabled {
		return nil, ErrOnlineDisabled
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = ProviderMock
	}
	if !s.IsProviderEnabled(cfg, provider) {
		return nil, ErrProviderDisabled
	}
	p, ok := s.Provider(provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	var amountCents int64
	var quota int64
	var currency string
	var pkgID *int

	if in.PackageID != nil && *in.PackageID > 0 {
		pkg, err := s.Client.RechargePackage.Query().
			Where(rechargepackage.IDEQ(*in.PackageID), rechargepackage.EnabledEQ(true)).
			Only(ctx)
		if err != nil {
			return nil, fmt.Errorf("package not found")
		}
		amountCents = pkg.AmountCents
		quota = pkg.QuotaAmount + pkg.BonusQuota
		currency = pkg.Currency
		id := pkg.ID
		pkgID = &id
	} else if in.CustomAmountCents != nil && *in.CustomAmountCents > 0 {
		amountCents = *in.CustomAmountCents
		currency = strings.TrimSpace(in.Currency)
		if currency == "" {
			currency = "CNY"
		}
		quota = QuotaForAmount(amountCents, currency, s.AppConfig)
	} else {
		return nil, ErrInvalidAmount
	}
	if amountCents < cfg.MinAmountCents || amountCents > cfg.MaxAmountCents {
		return nil, ErrInvalidAmount
	}
	if quota <= 0 {
		return nil, ErrInvalidAmount
	}

	orderNo := NewOrderNo()
	expireAt := time.Now().Add(DefaultOrderTTL * time.Second)
	snap := OrderSnapshot{
		OrderNo:     orderNo,
		AmountCents: amountCents,
		Currency:    currency,
		QuotaAmount: quota,
		UserID:      in.UserID,
	}
	res, err := p.CreatePayment(ctx, cfg, snap)
	if err != nil {
		return nil, err
	}

	cr := s.Client.PaymentOrder.Create().
		SetOrderNo(orderNo).
		SetUserID(in.UserID).
		SetProvider(provider).
		SetAmountCents(amountCents).
		SetCurrency(currency).
		SetQuotaAmount(quota).
		SetStatus(StatusPending).
		SetExpireAt(expireAt).
		SetInvoiceRequested(in.InvoiceRequested)
	if pkgID != nil {
		cr.SetPackageID(*pkgID)
	}
	if res.PayURL != "" {
		cr.SetPayURL(res.PayURL)
	}
	if res.QRCode != "" {
		cr.SetQrCode(res.QRCode)
	}
	if res.ProviderTxnID != "" {
		cr.SetNillableProviderTxnID(&res.ProviderTxnID)
	}
	if _, err := cr.Save(ctx); err != nil {
		return nil, err
	}
	return &CreateOrderOutput{
		OrderNo:     orderNo,
		Status:      StatusPending,
		PayURL:      res.PayURL,
		QRCode:      res.QRCode,
		ExpireAt:    expireAt,
		AmountCents: amountCents,
		QuotaAmount: quota,
	}, nil
}

var settleMu sync.Map

// SettleOrder 幂等入账。
func (s *Service) SettleOrder(ctx context.Context, orderNo, providerTxnID string, amountCents int64) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return ErrOrderNotFound
	}
	lockKey := orderNo
	if _, loaded := settleMu.LoadOrStore(lockKey, struct{}{}); loaded {
		for i := 0; i < 50; i++ {
			time.Sleep(20 * time.Millisecond)
			o, err := s.Client.PaymentOrder.Query().
				Where(paymentorder.OrderNoEQ(orderNo)).
				Only(ctx)
			if err == nil && o.Status == StatusPaid {
				return nil
			}
		}
	}
	defer settleMu.Delete(lockKey)

	o, err := s.Client.PaymentOrder.Query().
		Where(paymentorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		return ErrOrderNotFound
	}
	if o.Status == StatusPaid {
		return nil
	}
	if o.Status != StatusPending {
		return ErrOrderNotPending
	}
	if amountCents > 0 && amountCents != o.AmountCents {
		return fmt.Errorf("amount mismatch")
	}
	if providerTxnID == "" {
		providerTxnID = "txn-" + uuid.New().String()
	}

	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return err
	}
	aff, err := tx.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(o.ID),
			paymentorder.StatusEQ(StatusPending),
		).
		SetStatus(StatusPaid).
		SetPaidAt(time.Now()).
		SetNillableProviderTxnID(&providerTxnID).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if aff != 1 {
		_ = tx.Rollback()
		o2, _ := s.Client.PaymentOrder.Get(ctx, o.ID)
		if o2 != nil && o2.Status == StatusPaid {
			return nil
		}
		return ErrOrderNotPending
	}
	u, err := tx.User.UpdateOneID(o.UserID).AddBalance(o.QuotaAmount).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.PaymentLedger.Create().
		SetUserID(o.UserID).
		SetOrderID(o.ID).
		SetDelta(o.QuotaAmount).
		SetBalanceAfter(u.Balance).
		SetEntryType(LedgerRecharge).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	user, _ := s.Client.User.Get(ctx, o.UserID)
	if user != nil {
		NotifyBalanceUpdated(ctx, user, "balance.updated", map[string]any{
			"user_id":        user.ID,
			"order_no":       orderNo,
			"delta":          o.QuotaAmount,
			"balance_after":  u.Balance,
			"entry_type":     "payment_recharge",
			"amount_cents":   o.AmountCents,
		})
	}
	return nil
}

func (s *Service) CancelOrder(ctx context.Context, userID int, orderNo string) error {
	aff, err := s.Client.PaymentOrder.Update().
		Where(
			paymentorder.OrderNoEQ(orderNo),
			paymentorder.UserIDEQ(userID),
			paymentorder.StatusEQ(StatusPending),
		).
		SetStatus(StatusCancelled).
		Save(ctx)
	if err != nil {
		return err
	}
	if aff != 1 {
		return ErrOrderNotPending
	}
	return nil
}

func (s *Service) ExpirePendingOrders(ctx context.Context) (int, error) {
	now := time.Now()
	aff, err := s.Client.PaymentOrder.Update().
		Where(
			paymentorder.StatusEQ(StatusPending),
			paymentorder.ExpireAtLT(now),
		).
		SetStatus(StatusExpired).
		Save(ctx)
	return aff, err
}

func (s *Service) StartExpiryWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.ExpirePendingOrders(context.Background())
			}
		}
	}()
}

func (s *Service) OrderSnapshotFromEnt(o *ent.PaymentOrder) OrderSnapshot {
	txn := ""
	if o.ProviderTxnID != nil {
		txn = *o.ProviderTxnID
	}
	return OrderSnapshot{
		OrderNo:       o.OrderNo,
		AmountCents:   o.AmountCents,
		Currency:      o.Currency,
		QuotaAmount:   o.QuotaAmount,
		UserID:        o.UserID,
		ProviderTxnID: txn,
	}
}

// CreateRefund 管理员退款（扣回额度并调渠道退款 API）。
func (s *Service) CreateRefund(ctx context.Context, orderID int, amountCents int64, quotaRevoke int64, reason string, operatorID int) (*ent.PaymentRefund, error) {
	o, err := s.Client.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, ErrOrderNotFound
	}
	if o.Status != StatusPaid && o.Status != StatusPartialRefund {
		return nil, errors.New("order not refundable")
	}
	if amountCents <= 0 {
		amountCents = o.AmountCents
	}
	if quotaRevoke <= 0 {
		quotaRevoke = o.QuotaAmount
	}
	u, err := s.Client.User.Get(ctx, o.UserID)
	if err != nil {
		return nil, err
	}
	if u.Balance < quotaRevoke {
		return nil, ErrInsufficientBal
	}
	cfg, err := s.RuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	p, ok := s.Provider(o.Provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider")
	}
	refundNo := "RF" + NewOrderNo()[2:]
	res, err := p.Refund(ctx, cfg, s.OrderSnapshotFromEnt(o), refundNo, amountCents, reason)
	if err != nil {
		return nil, err
	}
	if res == nil || !res.Success {
		return nil, errors.New("provider refund failed")
	}

	tx, err := s.Client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	nu, err := tx.User.UpdateOneID(o.UserID).AddBalance(-quotaRevoke).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	newStatus := StatusRefunded
	if amountCents < o.AmountCents {
		newStatus = StatusPartialRefund
	}
	_, err = tx.PaymentOrder.UpdateOneID(o.ID).SetStatus(newStatus).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	_, err = tx.PaymentLedger.Create().
		SetUserID(o.UserID).
		SetOrderID(o.ID).
		SetDelta(-quotaRevoke).
		SetBalanceAfter(nu.Balance).
		SetEntryType(LedgerRefund).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	rc := tx.PaymentRefund.Create().
		SetOrderID(o.ID).
		SetRefundNo(refundNo).
		SetAmountCents(amountCents).
		SetQuotaRevoked(quotaRevoke).
		SetStatus(RefundSuccess).
		SetProviderRefundID(res.ProviderRefundID).
		SetReason(reason)
	if operatorID > 0 {
		rc.SetOperatorID(operatorID)
	}
	rf, err := rc.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	user, _ := s.Client.User.Get(ctx, o.UserID)
	if user != nil {
		NotifyBalanceUpdated(ctx, user, "balance.updated", map[string]any{
			"user_id":       user.ID,
			"order_no":      o.OrderNo,
			"delta":         -quotaRevoke,
			"balance_after": nu.Balance,
			"entry_type":    "payment_refund",
		})
	}
	return rf, nil
}
