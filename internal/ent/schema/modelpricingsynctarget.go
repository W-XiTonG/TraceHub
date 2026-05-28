package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelPricingSyncTarget 配置模型定价双向同步的外部目标（仅管理端使用）。
type ModelPricingSyncTarget struct {
	ent.Schema
}

func (ModelPricingSyncTarget) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(120).NotEmpty(),
		field.Enum("direction").
			Values("pull", "push", "both").
			Default("both"),
		field.String("base_url").MaxLen(500).NotEmpty(),
		field.String("pull_path").MaxLen(255).Default("/api/pricing/sync/export"),
		field.String("push_path").MaxLen(255).Default("/api/pricing/sync/import"),
		field.Enum("credential_mode").
			Values("none", "static", "channel").
			Default("static"),
		field.Text("api_key_encrypted").Optional().Sensitive(),
		field.Int("channel_id").Optional().Comment("credential_mode=channel 时复用渠道密钥"),
		field.JSON("extra_headers", map[string]string{}).Optional(),
		field.Enum("parse_mode").
			Values("tracehub_pricing_v1").
			Default("tracehub_pricing_v1"),
		field.JSON("scopes", []string{}).Default([]string{"global", "channel", "group"}),
		field.JSON("channel_filter", []int{}).Optional().Comment("空=全部渠道"),
		field.JSON("group_filter", []int{}).Optional().Comment("空=全部分组"),
		field.Enum("model_include_mode").
			Values("all", "allowlist", "denylist").
			Default("all"),
		field.JSON("model_allowlist", []string{}).Optional(),
		field.JSON("model_denylist", []string{}).Optional(),
		field.JSON("model_include_patterns", []string{}).Optional(),
		field.JSON("channel_ref_map", map[string]int{}).Optional(),
		field.JSON("group_ref_map", map[string]int{}).Optional(),
		field.Enum("merge_strategy").
			Values("upsert", "skip_existing", "replace_scoped").
			Default("upsert"),
		field.Bool("schedule_enabled").Default(false),
		field.Int("schedule_interval_min").Default(60).Comment("最小 60"),
		field.Bool("enabled").Default(true),
		field.Time("last_pull_at").Optional(),
		field.Bool("last_pull_ok").Default(false),
		field.Text("last_pull_message").Optional(),
		field.Time("last_push_at").Optional(),
		field.Bool("last_push_ok").Default(false),
		field.Text("last_push_message").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ModelPricingSyncTarget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("model_pricing_sync_targets").
			Field("channel_id").
			Unique(),
	}
}

func (ModelPricingSyncTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("schedule_enabled"),
		index.Fields("channel_id"),
	}
}
