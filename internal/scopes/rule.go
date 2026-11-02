package scopes

import (
	"context"
	"slices"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
)

// Common error messages.
const (
	ErrNoUser = "no user in context"
	//nolint:gosec // False positive.
	ErrNoAPIKey = "no api key in context"
)

// AlwaysDeny returns a rule that denies access if no user is in context.
func AlwaysDeny() privacy.QueryMutationRule {
	return privacy.ContextQueryMutationRule(func(ctx context.Context) error {
		return privacy.Deny
	})
}

// hasScope checks if a scope exists in the given scopes slice.
func hasScope(scopes []string, requiredScope string) bool {
	return slices.Contains(scopes, requiredScope)
}

// HasSystemScope reports whether the user has the required scope.
// Roles were removed, so every authenticated user has all permissions.
func HasSystemScope(user *ent.User, requiredScope ScopeSlug) bool {
	return user != nil
}

// getUserFromContext safely retrieves user from context.
func getUserFromContext(ctx context.Context) (*ent.User, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, privacy.Denyf(ErrNoUser)
	}

	return user, nil
}

// getAPIKeyFromContext safely retrieves API key from context.
func getAPIKeyFromContext(ctx context.Context) (*ent.APIKey, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return nil, privacy.Skipf(ErrNoAPIKey)
	}

	return apiKey, nil
}
