package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RedeemLedger 记录每次成功核销兑换码后的入账流水（便于对账与用户侧查询）。
type RedeemLedger struct {
	ent.Schema
}

func (RedeemLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("redeem_code_id"),
		field.Int64("amount"),
		field.Int64("balance_after"),
		field.String("code_hint").MaxLen(80).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (RedeemLedger) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("redeem_ledgers").
			Field("user_id").
			Unique().
			Required(),
		edge.From("redeem_code", RedeemCode.Type).
			Ref("redeem_ledgers").
			Field("redeem_code_id").
			Unique().
			Required(),
	}
}

func (RedeemLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("redeem_code_id", "created_at"),
	}
}
