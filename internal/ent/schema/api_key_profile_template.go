package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type APIKeyProfileTemplate struct {
	ent.Schema
}

func (APIKeyProfileTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (APIKeyProfileTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("api_key_profile_templates_by_name").
			Unique(),
	}
}

func (APIKeyProfileTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("Template name"),
		field.String("description").
			Default("").
			Comment("Template description"),
		field.JSON("profile", &objects.APIKeyProfile{}).
			Default(&objects.APIKeyProfile{}).
			Optional().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
	}
}

func (APIKeyProfileTemplate) Edges() []ent.Edge {
	return nil
}

func (APIKeyProfileTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (APIKeyProfileTemplate) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.UserReadScopeRule(scopes.ScopeReadAPIKeys),
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadAPIKeys),
			scopes.OwnerRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.UserWriteScopeRule(scopes.ScopeWriteAPIKeys),
			scopes.OwnerRule(),
		},
	}
}