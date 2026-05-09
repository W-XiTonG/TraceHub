package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type GroupModelRate struct {
	ent.Schema
}

func (GroupModelRate) Fields() []ent.Field {
	return []ent.Field{
		field.Int("group_id"),
		field.String("model_name").MaxLen(255).NotEmpty(),
		field.Float("input_rate").Default(1),
		field.Float("output_rate").Default(1),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (GroupModelRate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Ref("group_model_rates").
			Field("group_id").
			Unique().
			Required(),
	}
}

func (GroupModelRate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "model_name").Unique(),
		index.Fields("model_name"),
	}
}
