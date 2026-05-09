package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogSource 配置从官方或第三方拉取模型列表的 HTTP 源（仅管理端使用）。
type ModelCatalogSource struct {
	ent.Schema
}

func (ModelCatalogSource) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(120).NotEmpty(),
		field.String("provider_slug").MaxLen(64).Default("openai_compatible").Comment("展示/归类用"),
		field.String("base_url").MaxLen(500).NotEmpty().Comment("如 https://api.openai.com"),
		field.String("models_path").MaxLen(255).Default("/v1/models"),
		field.Enum("credential_mode").
			Values("none", "static", "channel").
			Default("static"),
		field.Text("api_key_encrypted").Optional().Sensitive().Comment("credential_mode=static 时使用 AES 密文"),
		field.Int("channel_id").Optional().Comment("credential_mode=channel 时复用渠道密钥与 Base"),
		field.JSON("extra_headers", map[string]string{}).Optional().Comment("附加请求头"),
		field.Enum("parse_mode").
			Values("openai_json").
			Default("openai_json"),
		field.Bool("enabled").Default(true),
		field.Time("last_sync_at").Optional(),
		field.Bool("last_sync_ok").Default(false),
		field.Text("last_sync_message").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ModelCatalogSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("model_catalog_sources").
			Field("channel_id").
			Unique(),
		edge.To("catalog_entries", ModelCatalogEntry.Type),
	}
}

func (ModelCatalogSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("channel_id"),
	}
}
