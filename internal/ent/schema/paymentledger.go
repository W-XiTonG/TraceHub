package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PaymentLedger 支付入账/退款流水。
type PaymentLedger struct {
	ent.Schema
}

func (PaymentLedger) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("order_id"),
		field.Int64("delta"),
		field.Int64("balance_after"),
		field.String("entry_type").MaxLen(32).Comment("recharge|refund"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (PaymentLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("order_id"),
	}
}
