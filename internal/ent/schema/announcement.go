package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Announcement struct {
	ent.Schema
}

func (Announcement) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").MaxLen(500).NotEmpty(),
		field.Text("content").NotEmpty(),
		field.Bool("published").Default(false),
		field.Time("published_at").Optional().Nillable(),
		field.Int("created_by").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Announcement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", User.Type).
			Ref("created_announcements").
			Field("created_by").
			Unique(),
	}
}

func (Announcement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("published"),
		index.Fields("published_at"),
	}
}
