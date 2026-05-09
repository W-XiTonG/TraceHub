package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Channel struct {
	ent.Schema
}

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.Int("group_id").Optional(),
		field.String("channel_type").MaxLen(50).NotEmpty(),
		field.String("channel_name").Optional().MaxLen(255),
		field.Enum("auth_type").
			Values("api_key", "oauth", "setup_token").
			Default("api_key"),
		field.Text("api_key").Optional().Sensitive(),
		field.Text("oauth_token").Optional().Sensitive(),
		field.Text("oauth_refresh_token").Optional().Sensitive(),
		field.Time("oauth_expires_at").Optional(),
		field.Text("setup_token").Optional().Sensitive(),
		field.String("api_base_url").Default("").Optional().MaxLen(500),
		field.JSON("models", []string{}).Default([]string{}),
		field.Int("priority").Default(0),
		field.Int("weight").Default(100),
		field.Int("concurrency").Default(1),
		field.Float("load_factor").Default(1),
		field.Bool("schedulable").Default(true),
		field.Bool("enabled").Default(true),
		field.Int("health_status").Default(1),
		field.Int("error_count").Default(0),
		field.Time("last_health_check").Optional(),
		field.Text("last_error_msg").Optional(),
		field.Int64("quota_used").Default(0),
		field.Int64("quota_total").Default(0),
		field.JSON("model_alias", map[string]string{}).Optional().Comment("request model -> upstream model"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Ref("channels").
			Field("group_id").
			Unique(),
		edge.To("usage_logs", UsageLog.Type),
	}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_type"),
		index.Fields("enabled"),
		index.Fields("health_status"),
		index.Fields("group_id"),
	}
}
