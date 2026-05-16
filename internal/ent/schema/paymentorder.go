package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PaymentOrder 支付订单。
type PaymentOrder struct {
	ent.Schema
}

func (PaymentOrder) Fields() []ent.Field {
	return []ent.Field{
		field.String("order_no").MaxLen(64).Unique().NotEmpty(),
		field.Int("user_id"),
		field.String("provider").MaxLen(32).NotEmpty().Comment("mock|alipay|wxpay|stripe|easypay"),
		field.Int64("amount_cents").NonNegative(),
		field.String("currency").Default("CNY").MaxLen(8),
		field.Int64("quota_amount").Positive(),
		field.String("status").Default("pending").MaxLen(32).Comment("pending|paid|expired|cancelled|refunding|refunded|partial_refund"),
		field.String("provider_txn_id").MaxLen(128).Optional().Nillable(),
		field.String("pay_url").MaxLen(2000).Optional(),
		field.String("qr_code").MaxLen(4000).Optional(),
		field.Int("package_id").Optional(),
		field.Bool("invoice_requested").Default(false),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.Time("paid_at").Optional().Nillable(),
		field.Time("expire_at"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (PaymentOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("payment_orders").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (PaymentOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("status", "created_at"),
		index.Fields("order_no").Unique(),
		index.Fields("provider_txn_id").Unique(),
	}
}
