package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogEntry 模型市场中的单条模型（可来自同步或后续手工维护）。
type ModelCatalogEntry struct {
	ent.Schema
}

func (ModelCatalogEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("model_id").MaxLen(255).NotEmpty().Unique(),
		field.String("display_name").MaxLen(255).Optional(),
		field.String("provider_slug").MaxLen(64).Default(""),
		field.String("owned_by").MaxLen(128).Optional(),
		field.Text("description").Optional(),
		field.Int("source_id").Optional(),
		field.JSON("metadata", map[string]any{}).Optional(),
		field.JSON("admin_labels", []string{}).Optional().Comment("后台配置的标签，多值；同步上游不覆盖"),
		field.Bool("visible").Default(true).Comment("在内部模型市场中展示"),
		field.Bool("enabled").Default(true),
		field.Time("synced_at").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ModelCatalogEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", ModelCatalogSource.Type).
			Ref("catalog_entries").
			Field("source_id").
			Unique(),
	}
}

func (ModelCatalogEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_id"),
		index.Fields("source_id"),
		index.Fields("visible"),
		index.Fields("provider_slug"),
	}
}
