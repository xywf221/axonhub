package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

// Prompt holds the schema definition for the Prompt entity.
type Prompt struct {
	ent.Schema
}

func (Prompt) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Prompt) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("prompts_by_name").
			Unique(),
	}
}

// Fields of the Prompt.
func (Prompt) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("prompt name"),
		field.String("description").
			Default("").
			Comment("prompt description"),
		field.String("role").
			Comment("prompt role"),
		field.String("content").
			Comment("prompt content").
			SchemaType(map[string]string{
				dialect.MySQL: "longtext",
			}),
		field.Enum("status").
			Values("enabled", "disabled").
			Default("disabled"),
		field.Int("order").
			Default(0).
			Comment("prompt insertion order, smaller values are inserted first").
			Annotations(entgql.OrderField("ORDER")),
		field.JSON("settings", objects.PromptSettings{}).
			Comment("prompt settings in JSON format"),
	}
}

// Edges of the Prompt.
func (Prompt) Edges() []ent.Edge {
	return nil
}

func (Prompt) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Prompt) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadPrompts),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWritePrompts),
		},
	}
}
