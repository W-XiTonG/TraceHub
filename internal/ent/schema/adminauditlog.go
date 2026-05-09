package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AdminAuditLog struct {
	ent.Schema
}

func (AdminAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("admin_id"),
		field.String("action").MaxLen(100).NotEmpty(),
		field.String("resource").MaxLen(100).Optional(),
		field.String("resource_id").MaxLen(64).Optional(),
		field.JSON("detail", map[string]any{}).Optional(),
		field.String("ip_address").Optional().MaxLen(64),
		field.Text("user_agent").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (AdminAuditLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("admin", User.Type).
			Ref("admin_audit_logs").
			Field("admin_id").
			Unique().
			Required(),
	}
}

func (AdminAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("admin_id", "created_at"),
		index.Fields("action"),
	}
}
