package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChannelModelPricing 渠道级模型计价：覆盖全站 GlobalModelPricing（可选字段）与默认 in/out 倍率；分组 GroupModelRate 优先。
type ChannelModelPricing struct {
	ent.Schema
}

func (ChannelModelPricing) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id"),
		field.String("model_name").MaxLen(255).NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Int64("quota_in_per_1k").Optional().Nillable().Comment("nil=不覆盖，回退全站/代码基准"),
		field.Int64("quota_out_per_1k").Optional().Nillable().Comment("nil=不覆盖，回退全站/代码基准"),
		field.Float("input_rate").Default(1),
		field.Float("output_rate").Default(1),
		field.Float("markup_percent").Optional().Nillable().Comment("nil=回退全站默认成本加成%"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ChannelModelPricing) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("channel_model_pricings").
			Field("channel_id").
			Unique().
			Required(),
	}
}

func (ChannelModelPricing) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "model_name").Unique(),
		index.Fields("model_name"),
	}
}
