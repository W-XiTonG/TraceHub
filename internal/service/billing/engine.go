package billing

import (
	"context"
	"fmt"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/user"
	"tracehub/internal/service/balancealert"

	"github.com/pkoukk/tiktoken-go"
)

type Engine struct {
	Client *ent.Client
	Alerts *balancealert.Notifier
}

func (e *Engine) multipliers(ctx context.Context, u *ent.User, model string) (groupMul, userMul, inRate, outRate float64, err error) {
	groupMul = 1
	userMul = 1
	inRate, outRate = 1, 1
	if u != nil {
		userMul = u.RateMultiplier
		if u.Edges.Group != nil {
			groupMul = u.Edges.Group.RateMultiplier
			if gmr := e.firstGroupModelRate(ctx, u.Edges.Group.ID, model); gmr != nil {
				inRate = gmr.InputRate
				outRate = gmr.OutputRate
			}
		}
	}
	return groupMul, userMul, inRate, outRate, nil
}

func (e *Engine) firstGroupModelRate(ctx context.Context, groupID int, model string) *ent.GroupModelRate {
	names := CandidateGroupModelRateNames(model)
	if len(names) == 0 {
		return nil
	}
	// 单次查询替代对每个候选依次 Only，避免未命中时多次顺序往返（预检+实扣会各调一次 multipliers）。
	list, err := e.Client.GroupModelRate.Query().
		Where(
			groupmodelrate.GroupIDEQ(groupID),
			groupmodelrate.ModelNameIn(names...),
			groupmodelrate.Enabled(true),
		).All(ctx)
	if err != nil || len(list) == 0 {
		return nil
	}
	byName := make(map[string]*ent.GroupModelRate, len(list))
	for _, r := range list {
		byName[r.ModelName] = r
	}
	for _, name := range names {
		if gmr, ok := byName[name]; ok {
			return gmr
		}
	}
	return nil
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

// ModelBillRates 缓存一次 LookupPricing + multipliers（含 GroupModelRate 查询）结果；
// 同一次请求内预检余额与实扣可复用，避免对同一 (用户, model) 重复打库。
type ModelBillRates struct {
	groupMul, userMul, inRate, outRate float64
	in1k, out1k                       int64
}

// ModelBillRates 拉取该用户在指定 logical model 下的计价因子（与 Charge 一致）。
func (e *Engine) ModelBillRates(ctx context.Context, u *ent.User, model string) (ModelBillRates, error) {
	in1k, out1k := LookupPricing(model)
	gm, um, ir, or, err := e.multipliers(ctx, u, model)
	if err != nil {
		return ModelBillRates{}, err
	}
	return ModelBillRates{groupMul: gm, userMul: um, inRate: ir, outRate: or, in1k: in1k, out1k: out1k}, nil
}

// Cost 与 ChargeWithPromptCache 数值一致（含 cached prompt 分层与向上取整）。
func (b ModelBillRates) Cost(promptTokens, completionTokens, cachedPromptTokens int) int64 {
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
