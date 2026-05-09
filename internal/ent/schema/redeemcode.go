package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RedeemCode struct {
	ent.Schema
}

func (RedeemCode) Fields() []ent.Field {
	return []ent.Field{
		field.Int("used_by").Optional(),
		field.Int("created_by").Optional(),
		field.String("code").MaxLen(255).Unique().NotEmpty(),
		field.String("code_type").MaxLen(50).NotEmpty(),
		field.Int64("amount").Default(0),
		field.Float("discount_rate").Default(1),
		field.Int64("min_amount").Default(0),
		field.Int("status").Default(1),
		field.Int("max_uses").Default(1),
		field.Int("used_count").Default(0),
		field.Time("valid_from").Optional(),
		field.Time("valid_until").Optional(),
		field.Time("used_at").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RedeemCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("used_by_user", User.Type).
			Ref("redeemed_codes").
			Field("used_by").
			Unique(),
		edge.From("creator", User.Type).
			Ref("created_redeem_codes").
			Field("created_by").
			Unique(),
	}
}

func (RedeemCode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code"),
		index.Fields("status"),
	}
}
