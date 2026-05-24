package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AdminGatewayAlert struct {
	ent.Schema
}

func (AdminGatewayAlert) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_kind").MaxLen(32).NotEmpty(),
		field.String("severity").MaxLen(16).Default("warn"),
		field.String("title").MaxLen(500).NotEmpty(),
		field.Text("summary").Optional(),
		field.JSON("payload_json", map[string]any{}).Optional(),
		field.String("request_id").MaxLen(64).Optional(),
		field.Int("user_id").Optional(),
		field.Int("status_code").Optional(),
		field.String("model").MaxLen(255).Optional(),
		field.Int("channel_id").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (AdminGatewayAlert) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("event_kind", "created_at"),
	}
}
