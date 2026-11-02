package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserOwnedQueryRule allows any authenticated user to query user-owned resources.
func UserOwnedQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		if _, err := getUserFromContext(ctx); err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allowf("Authenticated user can query")
	})
}

// UserOwnedMutationRule allows any authenticated user to modify user-owned resources.
func UserOwnedMutationRule() privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		if _, err := getUserFromContext(ctx); err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allowf("Authenticated user can mutate")
	})
}
