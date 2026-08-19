package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountAllowedUser holds the edge schema definition for the account_allowed_users
// relationship. 账号级用户白名单：账号分配给分组后，仅白名单内的用户可以使用
// 该账号；分组内新增的其他人不受影响（只是用不到这个账号）。
type AccountAllowedUser struct {
	ent.Schema
}

func (AccountAllowedUser) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_allowed_users"},
		// Composite primary key: (account_id, user_id).
		field.ID("account_id", "user_id"),
	}
}

func (AccountAllowedUser) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("user_id"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AccountAllowedUser) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
		edge.To("user", User.Type).
			Unique().
			Required().
			Field("user_id"),
	}
}

func (AccountAllowedUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}