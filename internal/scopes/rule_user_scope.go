package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserReadScopeRule allows any authenticated user to read.
func UserReadScopeRule(readScope ScopeSlug) privacy.QueryRule {
	return userScopeQueryRule{}
}

// userScopeQueryRule custom QueryRule implementation.
type userScopeQueryRule struct{}

func (r userScopeQueryRule) EvalQuery(ctx context.Context, q ent.Query) error {
	if _, err := getUserFromContext(ctx); err != nil {
		return privacy.Skipf("User not found in context")
	}

	return privacy.Allow
}

// UserWriteScopeRule allows any authenticated user to write.
func UserWriteScopeRule(writeScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		if _, err := getUserFromContext(ctx); err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allow
	})
}

// UserScopeQueryMutationRule allows any authenticated user to access.
func UserScopeQueryMutationRule(requiredScope ScopeSlug) privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		if _, err := getUserFromContext(ctx); err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allow
	})
}

func UserHasScope(ctx context.Context, requiredScope ScopeSlug) bool {
	user, ok := contexts.GetUser(ctx)
	return ok && user != nil
}
