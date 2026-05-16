package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// InvoiceProfile 用户发票抬头。
type InvoiceProfile struct {
	ent.Schema
}

func (InvoiceProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Unique(),
		field.String("title_type").Default("company").MaxLen(16).Comment("company|personal"),
		field.String("company_name").MaxLen(255).Optional(),
		field.String("tax_id").MaxLen(64).Optional(),
		field.String("address").MaxLen(500).Optional(),
		field.String("email").MaxLen(255).Optional(),
		field.String("phone").MaxLen(32).Optional(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (InvoiceProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("invoice_profile").
			Field("user_id").
			Unique().
			Required(),
	}
}
