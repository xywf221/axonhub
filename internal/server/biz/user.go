package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type UserServiceParams struct {
	fx.In

	CacheConfig   xcache.Config
	Ent           *ent.Client
	APIKeyService *APIKeyService
}

type UserService struct {
	*AbstractService

	UserCache     xcache.Cache[ent.User]
	apiKeyService *APIKeyService
}

func NewUserService(params UserServiceParams) *UserService {
	return &UserService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		UserCache:     xcache.NewFromConfig[ent.User](params.CacheConfig),
		apiKeyService: params.APIKeyService,
	}
}

// CreateUser creates a new user with hashed password.
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	client := s.entFromContext(ctx)

	// Hash the password
	var hashedPassword string
	if input.Password == OIDC_ONLY_PLACEHOLDER {
		hashedPassword = OIDC_ONLY_PLACEHOLDER
	} else {
		var err error

		hashedPassword, err = HashPassword(input.Password)
		if err != nil {
			return nil, err
		}
	}

	mut := client.User.Create().
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetEmail(input.Email).
		SetPassword(hashedPassword)

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// UpdateUser updates an existing user.
func (s *UserService) UpdateUser(ctx context.Context, id int, input ent.UpdateUserInput) (*ent.User, error) {
	client := s.entFromContext(ctx)

	mut := client.User.UpdateOneID(id).
		SetNillableEmail(input.Email).
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetNillableIsOwner(input.IsOwner).
		SetNillablePreferLanguage(input.PreferLanguage)

	if input.ClearAvatar {
		mut.ClearAvatar()
	} else {
		mut.SetNillableAvatar(input.Avatar)
	}

	if input.Password != nil {
		hashedPassword, err := HashPassword(*input.Password)
		if err != nil {
			return nil, err
		}

		mut.SetPassword(hashedPassword)
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate cache
	s.invalidateUserCache(ctx, id)

	return user, nil
}

// UpdateOwnProfile updates fields users are allowed to change for their own account.
func (s *UserService) UpdateOwnProfile(ctx context.Context, input ent.UpdateUserInput) (*ent.User, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	id := currentUser.ID

	return authz.RunWithSystemBypass(ctx, "update-own-profile", func(ctx context.Context) (*ent.User, error) {
		client := s.entFromContext(ctx)

		mut := client.User.UpdateOneID(id).
			SetNillableFirstName(input.FirstName).
			SetNillableLastName(input.LastName).
			SetNillablePreferLanguage(input.PreferLanguage)

		if input.ClearAvatar {
			mut.ClearAvatar()
		} else {
			mut.SetNillableAvatar(input.Avatar)
		}

		if input.Password != nil {
			hashedPassword, err := HashPassword(*input.Password)
			if err != nil {
				return nil, err
			}

			mut.SetPassword(hashedPassword)
		}

		user, err := mut.Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update user profile: %w", err)
		}

		// Invalidate cache
		s.invalidateUserCache(ctx, id)

		return user, nil
	})
}

// UpdateUserStatus updates the status of a user.
func (s *UserService) UpdateUserStatus(ctx context.Context, id int, status user.Status) (*ent.User, error) {
	var updatedUser *ent.User

	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		result, err := client.User.UpdateOneID(id).
			SetStatus(status).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update user status: %w", err)
		}
		updatedUser = result

		if status == user.StatusDeactivated {
			if err := s.apiKeyService.disablePersonalAPIKeysByUser(ctx, id); err != nil {
				return fmt.Errorf("failed to disable user personal API keys: %w", err)
			}
		}

		runAfterCommit(ctx, func(ctx context.Context) {
			s.invalidateUserCache(ctx, id)
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// GetUserByID gets a user by ID with caching.
func (s *UserService) GetUserByID(ctx context.Context, id int) (*ent.User, error) {
	// Try cache first
	cacheKey := buildUserCacheKey(id)
	if user, err := s.UserCache.Get(ctx, cacheKey); err == nil {
		return &user, nil
	}

	// Query database
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	user, err := client.User.Query().
		Where(user.IDEQ(id)).
		WithOidcIdentities().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the user
	// TODO: handle role scope changed.
	err = s.UserCache.Set(ctx, cacheKey, *user)
	if err != nil {
		log.Warn(ctx, "failed to cache user", zap.Error(err))
	}

	return user, nil
}

func buildUserCacheKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

// invalidateUserCache removes a user from cache.
func (s *UserService) invalidateUserCache(ctx context.Context, id int) {
	cacheKey := buildUserCacheKey(id)
	_ = s.UserCache.Delete(ctx, cacheKey)
}

// clearUserCache clears all user cache.
func (s *UserService) clearUserCache(ctx context.Context) {
	_ = s.UserCache.Clear(ctx)
}

// ConvertUserToUserInfo converts ent.User to objects.UserInfo.
// Note: This function panics if the provided user is nil.
func ConvertUserToUserInfo(ctx context.Context, u *ent.User) *objects.UserInfo {
	oidcIdentities := make([]objects.OIDCIdentityInfo, 0, len(u.Edges.OidcIdentities))
	for _, identity := range u.Edges.OidcIdentities {
		oidcIdentities = append(oidcIdentities, objects.OIDCIdentityInfo{
			ID:      objects.GUID{Type: ent.TypeOIDCIdentity, ID: identity.ID},
			IdpName: identity.IdpName,
			Issuer:  identity.Issuer,
			Subject: identity.Subject,
			Email:   identity.Email,
		})
	}

	return &objects.UserInfo{
		ID:             objects.GUID{Type: ent.TypeUser, ID: u.ID},
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		IsOwner:        u.IsOwner,
		PreferLanguage: u.PreferLanguage,
		Avatar:         &u.Avatar,
		OIDCIdentities: oidcIdentities,
		HasPassword:    u.Password != OIDC_ONLY_PLACEHOLDER,
	}
}

// DeleteUser soft deletes a user and archives their personal API keys.
func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	return s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		u, err := client.User.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		// Cannot delete owner users
		if u.IsOwner {
			return fmt.Errorf("cannot delete owner user, transfer ownership first")
		}

		if err := s.apiKeyService.archivePersonalAPIKeysByUser(ctx, id); err != nil {
			return fmt.Errorf("failed to archive user personal API keys: %w", err)
		}

		if err := client.User.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		s.invalidateUserCache(ctx, id)

		return nil
	})
}
