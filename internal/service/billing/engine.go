package billing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/globalmodelpricing"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/balancealert"

	"github.com/pkoukk/tiktoken-go"
)

type Engine struct {
	Client *ent.Client
	Alerts *balancealert.Notifier
	Config *config.Config

	noBillMu      sync.Mutex
	noBillList    []string
	noBillExpires time.Time

	globalPricingMu      sync.Mutex
	globalPricingByModel map[string]*ent.GlobalModelPricing
	globalPricingExpires time.Time

	defaultMarkupMu      sync.Mutex
	defaultMarkup        float64
	defaultMarkupExpires time.Time

	groupRatesMu      sync.Mutex
	groupRatesByGroup map[int]map[string]*ent.GroupModelRate
	groupRatesExpires map[int]time.Time
}

const noBillListCacheTTL = 60 * time.Second

const globalModelPricingCacheTTL = 60 * time.Second

const defaultMarkupCacheTTL = 60 * time.Second

const groupModelRateCacheTTL = 60 * time.Second

func (e *Engine) groupUserMultipliers(u *ent.User) (groupMul, userMul float64) {
	groupMul, userMul = 1, 1
	if u == nil {
		return
	}
	userMul = u.RateMultiplier
	if u.Edges.Group != nil {
		groupMul = u.Edges.Group.RateMultiplier
	}
	return
}

func (e *Engine) cachedGlobalPricingByModel(ctx context.Context) map[string]*ent.GlobalModelPricing {
	if e.Client == nil {
		return nil
	}
	now := time.Now()
	e.globalPricingMu.Lock()
	if now.Before(e.globalPricingExpires) && e.globalPricingByModel != nil {
		m := e.globalPricingByModel
		e.globalPricingMu.Unlock()
		return m
	}
	e.globalPricingMu.Unlock()

	list, err := e.Client.GlobalModelPricing.Query().Where(globalmodelpricing.EnabledEQ(true)).All(ctx)
	if err != nil {
		e.globalPricingMu.Lock()
		prev := e.globalPricingByModel
		e.globalPricingMu.Unlock()
		if prev != nil {
			return prev
		}
		return nil
	}
	byName := make(map[string]*ent.GlobalModelPricing, len(list))
	for _, row := range list {
		byName[row.ModelName] = row
	}
	e.globalPricingMu.Lock()
	e.globalPricingByModel = byName
	e.globalPricingExpires = now.Add(globalModelPricingCacheTTL)
	e.globalPricingMu.Unlock()
	return byName
}

func (e *Engine) lookupGlobalModelPricingRow(ctx context.Context, model string) *ent.GlobalModelPricing {
	byName := e.cachedGlobalPricingByModel(ctx)
	if len(byName) == 0 {
		return nil
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if row, ok := byName[name]; ok {
			return row
		}
	}
	return nil
}

// InvalidateGlobalModelPricingCache 管理端更新全局模型计价后调用。
func (e *Engine) InvalidateGlobalModelPricingCache() {
	e.globalPricingMu.Lock()
	e.globalPricingByModel = nil
	e.globalPricingExpires = time.Time{}
	e.globalPricingMu.Unlock()
}

func (e *Engine) cachedDefaultPricingMarkupPercent(ctx context.Context) float64 {
	if e.Client == nil {
		if e.Config != nil {
			return e.Config.Site.DefaultPricingMarkupPercent
		}
		return 0
	}
	now := time.Now()
	e.defaultMarkupMu.Lock()
	if now.Before(e.defaultMarkupExpires) {
		m := e.defaultMarkup
		e.defaultMarkupMu.Unlock()
		return m
	}
	e.defaultMarkupMu.Unlock()

	m := sitecfg.EffectiveDefaultPricingMarkupPercent(ctx, e.Client, e.Config)
	e.defaultMarkupMu.Lock()
	e.defaultMarkup = m
	e.defaultMarkupExpires = now.Add(defaultMarkupCacheTTL)
	e.defaultMarkupMu.Unlock()
	return m
}

// InvalidateDefaultPricingMarkupCache 管理端更新 site.default_pricing_markup_percent 后调用。
func (e *Engine) InvalidateDefaultPricingMarkupCache() {
	e.defaultMarkupMu.Lock()
	e.defaultMarkupExpires = time.Time{}
	e.defaultMarkupMu.Unlock()
}

func (e *Engine) cachedGroupModelRatesByName(ctx context.Context, groupID int) map[string]*ent.GroupModelRate {
	if e.Client == nil || groupID <= 0 {
		return nil
	}
	now := time.Now()
	e.groupRatesMu.Lock()
	if e.groupRatesByGroup != nil {
		if exp, ok := e.groupRatesExpires[groupID]; ok && now.Before(exp) {
			m := e.groupRatesByGroup[groupID]
			e.groupRatesMu.Unlock()
			return m
		}
	}
	e.groupRatesMu.Unlock()

	list, err := e.Client.GroupModelRate.Query().
		Where(groupmodelrate.GroupIDEQ(groupID)).
		All(ctx)
	if err != nil {
		e.groupRatesMu.Lock()
		prev := e.groupRatesByGroup[groupID]
		e.groupRatesMu.Unlock()
		return prev
	}
	byName := make(map[string]*ent.GroupModelRate, len(list))
	for _, r := range list {
		byName[r.ModelName] = r
	}
	e.groupRatesMu.Lock()
	if e.groupRatesByGroup == nil {
		e.groupRatesByGroup = make(map[int]map[string]*ent.GroupModelRate)
		e.groupRatesExpires = make(map[int]time.Time)
	}
	e.groupRatesByGroup[groupID] = byName
	e.groupRatesExpires[groupID] = now.Add(groupModelRateCacheTTL)
	e.groupRatesMu.Unlock()
	return byName
}

// InvalidateGroupModelRateCache 管理端更新某分组模型倍率后调用；groupID<=0 时清空全部分组缓存。
func (e *Engine) InvalidateGroupModelRateCache(groupID int) {
	e.groupRatesMu.Lock()
	defer e.groupRatesMu.Unlock()
	if groupID <= 0 {
		e.groupRatesByGroup = nil
		e.groupRatesExpires = nil
		return
	}
	delete(e.groupRatesByGroup, groupID)
	delete(e.groupRatesExpires, groupID)
}

func (e *Engine) lookupGroupModelRateRow(ctx context.Context, groupID int, model string) *ent.GroupModelRate {
	names := CandidateGroupModelRateNames(model)
	if len(names) == 0 {
		return nil
	}
	byName := e.cachedGroupModelRatesByName(ctx, groupID)
	if len(byName) == 0 {
		return nil
	}
	for _, name := range names {
		if gmr, ok := byName[name]; ok {
			return gmr
		}
	}
	return nil
}

func (e *Engine) modelBillable(ctx context.Context, u *ent.User, model string) bool {
	if u != nil && u.Edges.Group != nil {
		if gmr := e.lookupGroupModelRateRow(ctx, u.Edges.Group.ID, model); gmr != nil {
			return gmr.Billable
		}
	}
	return !modelMatchesNoBillList(model, e.cachedNoBillModels(ctx))
}

func modelMatchesNoBillList(model string, list []string) bool {
	if len(list) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(list))
	for _, id := range list {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = struct{}{}
		}
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if _, ok := set[name]; ok {
			return true
		}
	}
	return false
}

func (e *Engine) cachedNoBillModels(ctx context.Context) []string {
	if e.Client == nil {
		return nil
	}
	cfg := e.Config
	now := time.Now()
	e.noBillMu.Lock()
	if now.Before(e.noBillExpires) && e.noBillList != nil {
		list := append([]string(nil), e.noBillList...)
		e.noBillMu.Unlock()
		return list
	}
	e.noBillMu.Unlock()

	list := sitecfg.EffectiveNoBillModels(ctx, e.Client, cfg)
	e.noBillMu.Lock()
	e.noBillList = list
	e.noBillExpires = now.Add(noBillListCacheTTL)
	e.noBillMu.Unlock()
	return list
}

// InvalidateNoBillModelsCache 管理端更新 site.no_bill_models 后调用。
func (e *Engine) InvalidateNoBillModelsCache() {
	e.noBillMu.Lock()
	e.noBillList = nil
	e.noBillExpires = time.Time{}
	e.noBillMu.Unlock()
}

func (e *Engine) EstimateTokens(model string, promptText string) (int, error) {
	encoding := "cl100k_base"
	if strings.HasPrefix(model, "gpt-4o") {
		encoding = "o200k_base"
	}
	tke, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return len(promptText) / 4, nil
	}
	toks := tke.Encode(promptText, nil, nil)
	return len(toks), nil
}

func (e *Engine) Charge(ctx context.Context, u *ent.User, model string, promptTokens, completionTokens int) (cost int64, err error) {
	return e.chargeTokens(ctx, u, model, promptTokens, completionTokens, 0)
}

// ChargeWithPromptCache bills prompt_tokens with a reduced rate for cached_prompt subset (OpenAI-style prompt cache).
func (e *Engine) ChargeWithPromptCache(ctx context.Context, u *ent.User, model string, promptTokens, completionTokens, cachedPromptTokens int) (cost int64, err error) {
	return e.chargeTokens(ctx, u, model, promptTokens, completionTokens, cachedPromptTokens)
}

// ModelBillRates 缓存一次 LookupPricing、全局/分组倍率与 billable 判定结果；
// 同一次请求内预检余额与实扣可复用，避免对同一 (用户, model) 重复打库。
type ModelBillRates struct {
	groupMul, userMul, inRate, outRate float64
	in1k, out1k                       int64
	markupPercent                     float64
	Billable                         bool
}

// ModelBillRates 拉取该用户在指定 logical model 下的计价因子（与 Charge 一致）。
// 基准价：代码内建 LookupPricing，可被全局 GlobalModelPricing 的非空 quota 覆盖（分组暂无基准价字段）。
// in/out 倍率：分组 GroupModelRate（enabled）优先；否则全局 GlobalModelPricing（enabled）；否则 1。
func (e *Engine) ModelBillRates(ctx context.Context, u *ent.User, model string) (ModelBillRates, error) {
	in1k, out1k := LookupPricing(model)
	glo := e.lookupGlobalModelPricingRow(ctx, model)
	if glo != nil && glo.Enabled {
		if glo.QuotaInPer1k != nil {
			in1k = *glo.QuotaInPer1k
		}
		if glo.QuotaOutPer1k != nil {
			out1k = *glo.QuotaOutPer1k
		}
	}
	gm, um := e.groupUserMultipliers(u)
	ir, or := 1.0, 1.0
	if u != nil && u.Edges.Group != nil {
		if gmr := e.lookupGroupModelRateRow(ctx, u.Edges.Group.ID, model); gmr != nil && gmr.Enabled {
			ir, or = gmr.InputRate, gmr.OutputRate
		} else if glo != nil && glo.Enabled {
			ir, or = glo.InputRate, glo.OutputRate
		}
	} else if glo != nil && glo.Enabled {
		ir, or = glo.InputRate, glo.OutputRate
	}
	siteMarkup := e.cachedDefaultPricingMarkupPercent(ctx)
	markup := ResolveMarkupPercent(siteMarkup, glo)
	return ModelBillRates{
		groupMul: gm, userMul: um, inRate: ir, outRate: or,
		in1k: in1k, out1k: out1k, markupPercent: markup,
		Billable: e.modelBillable(ctx, u, model),
	}, nil
}

// EffectiveQuotaPer1k 返回对该用户生效的每千 token 额度（入/出），与 Cost(1000,0,0)、Cost(0,1000,0) 一致。
func (b ModelBillRates) EffectiveQuotaPer1k() (in, out int64) {
	if !b.Billable {
		return 0, 0
	}
	return b.Cost(1000, 0, 0), b.Cost(0, 1000, 0)
}

// Cost 与 ChargeWithPromptCache 数值一致（含 cached prompt 分层与向上取整）。
func (b ModelBillRates) Cost(promptTokens, completionTokens, cachedPromptTokens int) int64 {
	if !b.Billable {
		return 0
	}
	if cachedPromptTokens < 0 {
		cachedPromptTokens = 0
	}
	if cachedPromptTokens > promptTokens {
		cachedPromptTokens = promptTokens
	}
	uncachedPrompt := promptTokens - cachedPromptTokens
	cf := CachedPromptInputFactor
	raw := float64(uncachedPrompt)*float64(b.in1k)/1000.0*b.inRate +
		float64(cachedPromptTokens)*float64(b.in1k)/1000.0*b.inRate*cf +
		float64(completionTokens)*float64(b.out1k)/1000.0*b.outRate
	raw *= 1 + b.markupPercent/100.0
	raw *= b.groupMul * b.userMul
	return int64(raw + 0.999999)
}

func (e *Engine) chargeTokens(ctx context.Context, u *ent.User, model string, promptTokens, completionTokens, cachedPromptTokens int) (cost int64, err error) {
	br, err := e.ModelBillRates(ctx, u, model)
	if err != nil {
		return 0, err
	}
	return br.Cost(promptTokens, completionTokens, cachedPromptTokens), nil
}

func (e *Engine) UserBalance(ctx context.Context, userID int) (int64, error) {
	u, err := e.Client.User.Get(ctx, userID)
	if err != nil {
		return 0, err
	}
	return u.Balance, nil
}

// CreditBalance 增加用户余额（视频任务精调退还等）。
func (e *Engine) CreditBalance(ctx context.Context, userID int64, amount int64) error {
	if amount <= 0 {
		return nil
	}
	_, err := e.Client.User.Update().
		Where(user.IDEQ(int(userID))).
		AddBalance(amount).
		Save(ctx)
	return err
}

func (e *Engine) DeductBalance(ctx context.Context, userID int64, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("billing: negative deduct amount")
	}
	if amount == 0 {
		return nil
	}
	aff, err := e.Client.User.Update().
		Where(user.IDEQ(int(userID)), user.BalanceGTE(amount)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return err
	}
	if aff == 0 {
		return ErrInsufficientBalance
	}
	if e.Alerts == nil {
		return nil
	}
	u2, err := e.Client.User.Get(ctx, int(userID))
	if err != nil {
		return nil
	}
	newBal := u2.Balance
	oldBal := newBal + amount
	e.Alerts.MaybeNotifyCrossBelow(ctx, u2, oldBal, newBal)
	return nil
}

func (e *Engine) LoadUserWithGroup(ctx context.Context, userID int) (*ent.User, error) {
	return e.Client.User.Query().
		Where(user.IDEQ(userID)).
		WithGroup().
		Only(ctx)
}
