package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RechargePackage 充值套餐（管理端配置）。
type RechargePackage struct {
	ent.Schema
}

func (RechargePackage) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(120).NotEmpty(),
		field.Int64("amount_cents").NonNegative(),
		field.Int64("quota_amount").Positive(),
		field.Int64("bonus_quota").Default(0),
		field.String("currency").Default("CNY").MaxLen(8),
		field.Int("sort").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RechargePackage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "sort"),
	}
}
