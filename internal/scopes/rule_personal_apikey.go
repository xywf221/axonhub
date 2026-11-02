package scopes

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/privacy"
)

// UserPersonalAPIKeyReadRule allows any authenticated user to view API keys.
func UserPersonalAPIKeyReadRule(requiredScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, q privacy.Filter) error {
		if _, err := getUserFromContext(ctx); err != nil {
			return privacy.Skipf("User not found in context")
		}

		return privacy.Allowf("Authenticated user can query API keys")
	})
}
