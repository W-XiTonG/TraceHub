package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// GlobalModelPricing 全站模型计价：覆盖代码内建基准价（可选维度）与默认 in/out 倍率；分组 GroupModelRate 优先。
type GlobalModelPricing struct {
	ent.Schema
}

func (GlobalModelPricing) Fields() []ent.Field {
	return []ent.Field{
		field.String("model_name").MaxLen(255).NotEmpty().Unique(),
		field.Bool("enabled").Default(true),
		field.Int64("quota_in_per_1k").Optional().Nillable().Comment("nil=不覆盖代码基准输入每千额度"),
		field.Int64("quota_out_per_1k").Optional().Nillable().Comment("nil=不覆盖代码基准输出每千额度"),
		field.Float("input_rate").Default(1),
		field.Float("output_rate").Default(1),
		field.Float("markup_percent").Optional().Nillable().Comment("nil=使用全站默认成本加成%; 扣费时在成本额度上乘 (1+m/100)"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (GlobalModelPricing) Edges() []ent.Edge { return nil }
