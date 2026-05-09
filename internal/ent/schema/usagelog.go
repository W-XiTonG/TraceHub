package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UsageLog struct {
	ent.Schema
}

func (UsageLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Optional(),
		field.Int("api_key_id").Optional(),
		field.Int("channel_id").Optional(),
		field.String("request_id").Optional().MaxLen(64),
		field.String("model").MaxLen(255).NotEmpty(),
		field.String("model_upstream").Optional().MaxLen(255),
		field.Int("prompt_tokens").Default(0),
		field.Int("completion_tokens").Default(0),
		field.Int("total_tokens").Default(0),
		field.Int64("input_cost").Default(0),
		field.Int64("output_cost").Default(0),
		field.Int64("total_cost").Default(0),
		field.Time("request_time").Default(time.Now),
		field.Int("response_time_ms").Optional(),
		field.Int("status_code").Optional(),
		field.Text("error_message").Optional(),
		field.String("ip_address").Optional().MaxLen(64),
		field.Text("user_agent").Optional(),
		field.Bool("is_cached").Default(false),
		field.Int("cached_tokens").Default(0),
	}
}

func (UsageLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("usage_logs").
			Field("user_id").
			Unique(),
		edge.From("api_key", APIKey.Type).
			Ref("usage_logs").
			Field("api_key_id").
			Unique(),
		edge.From("channel", Channel.Type).
			Ref("usage_logs").
			Field("channel_id").
			Unique(),
	}
}

func (UsageLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "request_time"),
		index.Fields("api_key_id"),
		index.Fields("channel_id"),
		index.Fields("model"),
		index.Fields("request_time"),
	}
}
