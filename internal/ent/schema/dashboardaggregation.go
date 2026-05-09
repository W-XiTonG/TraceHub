package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DashboardAggregation struct {
	ent.Schema
}

func (DashboardAggregation) Fields() []ent.Field {
	return []ent.Field{
		field.Time("agg_date"),
		field.String("agg_type").MaxLen(50).NotEmpty(),
		field.Int64("total_requests").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Int64("total_cost").Default(0),
		field.Int("active_users").Default(0),
		field.Int("new_users").Default(0),
		field.JSON("model_stats", map[string]any{}).Optional(),
		field.JSON("channel_stats", map[string]any{}).Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (DashboardAggregation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agg_date", "agg_type").Unique(),
	}
}
