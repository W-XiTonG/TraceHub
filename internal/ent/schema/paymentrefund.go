package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PaymentRefund 退款单。
type PaymentRefund struct {
	ent.Schema
}

func (PaymentRefund) Fields() []ent.Field {
	return []ent.Field{
		field.Int("order_id"),
		field.String("refund_no").MaxLen(64).Unique().NotEmpty(),
		field.Int64("amount_cents").NonNegative(),
		field.Int64("quota_revoked").NonNegative(),
		field.String("status").Default("pending").MaxLen(32).Comment("pending|success|failed"),
		field.String("provider_refund_id").MaxLen(128).Optional(),
		field.String("reason").MaxLen(500).Optional(),
		field.Int("operator_id").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (PaymentRefund) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id"),
		index.Fields("refund_no").Unique(),
	}
}
