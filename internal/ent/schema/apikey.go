package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type APIKey struct {
	ent.Schema
}

func (APIKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Int("group_id").Optional(),
		field.String("key_hash").MaxLen(255).Unique().NotEmpty(),
		field.String("key_prefix").Optional().MaxLen(32),
		field.String("name").Optional().MaxLen(100),
		field.JSON("ip_whitelist", []string{}).Optional(),
		field.JSON("ip_blacklist", []string{}).Optional(),
		field.Int64("daily_quota").Default(-1),
		field.Int64("monthly_quota").Default(-1),
		field.Int64("total_quota").Default(-1),
		field.Int("window_5h_count").Default(0),
		field.Int("window_1d_count").Default(0),
		field.Int("window_7d_count").Default(0),
		field.Int64("window_5h_tokens").Default(0),
		field.Int64("window_1d_tokens").Default(0),
		field.Int64("window_7d_tokens").Default(0),
		field.Time("window_reset_time").Optional(),
		field.Bool("enabled").Default(true),
		field.Bool("log_full_request").Default(false),
		field.Time("expires_at").Optional(),
		field.Time("last_used_at").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (APIKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("api_keys").
			Field("user_id").
			Unique().
			Required(),
		edge.From("group", Group.Type).
			Ref("api_keys").
			Field("group_id").
			Unique(),
		edge.To("usage_logs", UsageLog.Type),
	}
}

func (APIKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key_hash"),
		index.Fields("user_id"),
		index.Fields("enabled"),
	}
}
