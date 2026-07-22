package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdminPermission holds one resource grant for a limited administrator.
// Resource/action validity belongs to the service registry; the database keeps
// only the structural JSON-array invariant so migrations stay forwards-safe.
type AdminPermission struct {
	ent.Schema
}

func (AdminPermission) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (AdminPermission) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("resource").MaxLen(64),
		field.JSON("actions", []string{}),
	}
}

func (AdminPermission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("admin_permissions").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (AdminPermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "resource").Unique(),
		index.Fields("user_id"),
		index.Fields("resource"),
	}
}
