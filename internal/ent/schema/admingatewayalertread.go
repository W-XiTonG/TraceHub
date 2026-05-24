package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AdminGatewayAlertRead struct {
	ent.Schema
}

func (AdminGatewayAlertRead) Fields() []ent.Field {
	return []ent.Field{
		field.Int("admin_id").Unique(),
		field.Time("last_read_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (AdminGatewayAlertRead) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("admin", User.Type).
			Ref("gateway_alert_read_state").
			Field("admin_id").
			Unique().
			Required(),
	}
}

func (AdminGatewayAlertRead) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("admin_id"),
	}
}
