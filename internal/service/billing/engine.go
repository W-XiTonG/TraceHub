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
			gmr, err := e.Client.GroupModelRate.Query().
				Where(
					groupmodelrate.GroupIDEQ(u.Edges.Group.ID),
					groupmodelrate.ModelNameEQ(model),
					groupmodelrate.Enabled(true),
				).Only(ctx)
			if err == nil && gmr != nil {
				inRate = gmr.InputRate
				outRate = gmr.OutputRate
			}
		}
	}
	return groupMul, userMul, inRate, outRate, nil
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
	in1k, out1k := LookupPricing(model)
	gm, um, ir, or, err := e.multipliers(ctx, u, model)
	if err != nil {
		return 0, err
	}
	raw := float64(promptTokens)*float64(in1k)/1000.0*ir +
		float64(completionTokens)*float64(out1k)/1000.0*or
	raw *= gm * um
	return int64(raw + 0.999999), nil
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
