package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Group holds user/channel pool configuration.
type Group struct {
	ent.Schema
}

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).Unique().NotEmpty(),
		field.Text("description").Optional(),
		field.Float("rate_multiplier").Default(1),
		field.String("subscription_type").Default("pay_as_you_go").MaxLen(50),
		field.Int("rpm_limit").Default(0),
		field.JSON("model_routing", map[string][]int64{}).Optional(),
		field.String("routing_strategy").Optional().MaxLen(32).Comment("empty=inherit global; weighted_random|priority|fixed_order"),
		field.JSON("routing_options", map[string]any{}).Optional().Comment("per-group routing option overrides"),
		field.String("default_model").Optional().MaxLen(255),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type),
		edge.To("channels", Channel.Type),
		edge.To("group_model_rates", GroupModelRate.Type),
		edge.To("api_keys", APIKey.Type),
	}
}

func (Group) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name"),
	}
}
