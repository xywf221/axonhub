package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/privacy"
)

// OwnerRule allows authenticated users to access all functionality.
func OwnerRule() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		_, err := getUserFromContext(ctx)
		if err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allow
	})
}
