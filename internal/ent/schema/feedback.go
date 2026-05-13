package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Feedback 用户问题反馈；管理员可标记有效 Bug 并按额度发放奖励（入账用户 balance）。
type Feedback struct {
	ent.Schema
}

func (Feedback) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("preset_key").MaxLen(64).Default("").Comment("空表示自定义；非空须为服务端允许的预设 key"),
		field.String("title").MaxLen(200).NotEmpty(),
		field.Text("detail").NotEmpty(),
		field.Int("review_state").Default(0).Comment("0 待处理 1 无效 2 有效未发奖 3 已发奖"),
		field.Int64("reward_points").Default(0).Comment("管理员确认发放的额度；仅 review_state=3 时入账"),
		field.Int64("balance_after_reward").Optional().Nillable().Comment("发奖后用户余额快照"),
		field.Text("admin_note").Optional().MaxLen(4000).Comment("管理员备注（内部）"),
		field.Int("reviewed_by").Optional().Comment("处理人用户 id"),
		field.Time("reviewed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Feedback) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("feedbacks").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (Feedback) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("review_state", "created_at"),
	}
}
