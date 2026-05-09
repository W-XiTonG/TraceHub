package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type SystemConfig struct {
	ent.Schema
}

func (SystemConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("config_key").MaxLen(255).Unique().NotEmpty(),
		field.Text("config_value").Optional(),
		field.Text("description").Optional(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (SystemConfig) Indexes() []ent.Index { return nil }
