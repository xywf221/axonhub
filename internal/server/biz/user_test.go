package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestUserService(t *testing.T) (*UserService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	userService := &UserService{
		AbstractService: &AbstractService{db: client},
		UserCache:       xcache.NewFromConfig[ent.User](cacheConfig),
	}

	return userService, client
}

func setupTestUserAPIKeyService(t *testing.T, userService *UserService, client *ent.Client) *APIKeyService {
	t.Helper()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		CacheConfig: cacheConfig,
		Ent:         client,
		KeyPrefix:   "ah",
	})
	t.Cleanup(apiKeyService.Stop)
	userService.apiKeyService = apiKeyService

	return apiKeyService
}

func TestConvertUserToUserInfo_BasicUser(t *testing.T) {
	_, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a basic user
	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("hashed-password").
		SetFirstName("John").
		SetLastName("Doe").
		SetPreferLanguage("en").
		SetAvatar("https://example.com/avatar.jpg").
		SetIsOwner(false).
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Convert to UserInfo
	userInfo := ConvertUserToUserInfo(ctx, testUser)
	require.NotNil(t, userInfo)

	// Verify basic fields
	require.Equal(t, "test@example.com", userInfo.Email)
	require.Equal(t, "John", userInfo.FirstName)
	require.Equal(t, "Doe", userInfo.LastName)
	require.Equal(t, "en", userInfo.PreferLanguage)
	require.Equal(t, false, userInfo.IsOwner)
	require.NotNil(t, userInfo.Avatar)
	require.Equal(t, "https://example.com/avatar.jpg", *userInfo.Avatar)
	require.True(t, userInfo.HasPassword)
}

func TestConvertUserToUserInfo_NilUser(t *testing.T) {
	// Test with nil user
	require.Panics(t, func() {
		ConvertUserToUserInfo(context.Background(), nil)
	})
}

func TestUpdateUser_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Load user into cache
	cachedUser, err := userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)
	require.NotNil(t, cachedUser)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Update user
	newEmail := "newemail@example.com"
	input := ent.UpdateUserInput{
		Email: &newEmail,
	}
	_, err = userService.UpdateUser(ctx, testUser.ID, input)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after update")
}

func TestUpdateUserStatus_CacheInvalidation(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()
	setupTestUserAPIKeyService(t, userService, client)

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a user
	testUser, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Load user into cache
	_, err = userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)

	// Verify user is in cache
	cacheKey := buildUserCacheKey(testUser.ID)
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.NoError(t, err, "User should be in cache")

	// Update user status
	_, err = userService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)

	// Verify cache was invalidated
	_, err = userService.UserCache.Get(ctx, cacheKey)
	require.Error(t, err, "User cache should be invalidated after status update")
}

func TestUpdateUserStatusDeactivatedDisablesOnlyPersonalAPIKeys(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()
	apiKeyService := setupTestUserAPIKeyService(t, userService, client)

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	testUser, err := client.User.Create().
		SetEmail("deactivate@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	personalKey, err := client.APIKey.Create().
		SetKey("ah-deactivate-personal-key").
		SetName("Deactivate Personal Key").
		SetType(apikey.TypePersonal).
		SetUser(testUser).
		Save(ctx)
	require.NoError(t, err)

	userKey, err := client.APIKey.Create().
		SetKey("ah-deactivate-user-key").
		SetName("Deactivate User Key").
		SetType(apikey.TypeUser).
		SetUser(testUser).
		Save(ctx)
	require.NoError(t, err)

	serviceKey, err := client.APIKey.Create().
		SetKey("ah-deactivate-service-key").
		SetName("Deactivate Service Key").
		SetType(apikey.TypeServiceAccount).
		SetUser(testUser).
		Save(ctx)
	require.NoError(t, err)

	cachedKey, err := apiKeyService.GetAPIKey(ctx, personalKey.Key)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, cachedKey.Status)

	updatedUser, err := userService.UpdateUserStatus(ctx, testUser.ID, user.StatusDeactivated)
	require.NoError(t, err)
	require.Equal(t, user.StatusDeactivated, updatedUser.Status)

	disabledPersonalKey, err := client.APIKey.Get(ctx, personalKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusDisabled, disabledPersonalKey.Status)

	unchangedUserKey, err := client.APIKey.Get(ctx, userKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, unchangedUserKey.Status)

	unchangedServiceKey, err := client.APIKey.Get(ctx, serviceKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, unchangedServiceKey.Status)

	refreshedKey, err := apiKeyService.GetAPIKey(ctx, personalKey.Key)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusDisabled, refreshedKey.Status)

	// Reactivating the user must not silently re-enable revoked personal keys.
	_, err = userService.UpdateUserStatus(ctx, testUser.ID, user.StatusActivated)
	require.NoError(t, err)
	personalKeyAfterReactivation, err := client.APIKey.Get(ctx, personalKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusDisabled, personalKeyAfterReactivation.Status)
}

func TestDeleteUserArchivesOnlyPersonalAPIKeysAndInvalidatesCache(t *testing.T) {
	userService, client := setupTestUserService(t)
	defer client.Close()
	apiKeyService := setupTestUserAPIKeyService(t, userService, client)

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	targetUser, err := client.User.Create().
		SetEmail("target@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	otherUser, err := client.User.Create().
		SetEmail("other@example.com").
		SetPassword("password").
		Save(ctx)
	require.NoError(t, err)

	targetKey, err := client.APIKey.Create().
		SetKey("ah-target-key").
		SetName("Target Key").
		SetType(apikey.TypePersonal).
		SetUser(targetUser).
		Save(ctx)
	require.NoError(t, err)

	disabledPersonalKey, err := client.APIKey.Create().
		SetKey("ah-target-disabled-personal-key").
		SetName("Target Disabled Personal Key").
		SetType(apikey.TypePersonal).
		SetStatus(apikey.StatusDisabled).
		SetUser(targetUser).
		Save(ctx)
	require.NoError(t, err)

	targetUserKey, err := client.APIKey.Create().
		SetKey("ah-target-user-key").
		SetName("Target User Key").
		SetType(apikey.TypeUser).
		SetUser(targetUser).
		Save(ctx)
	require.NoError(t, err)

	targetServiceKey, err := client.APIKey.Create().
		SetKey("ah-target-service-key").
		SetName("Target Service Key").
		SetType(apikey.TypeServiceAccount).
		SetUser(targetUser).
		Save(ctx)
	require.NoError(t, err)

	otherKey, err := client.APIKey.Create().
		SetKey("ah-other-key").
		SetName("Other Key").
		SetUser(otherUser).
		Save(ctx)
	require.NoError(t, err)

	// Prime the cache with the enabled value before deleting the user.
	cachedKey, err := apiKeyService.GetAPIKey(ctx, targetKey.Key)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, cachedKey.Status)

	require.NoError(t, userService.DeleteUser(ctx, targetUser.ID))

	archivedKey, err := client.APIKey.Get(ctx, targetKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusArchived, archivedKey.Status)

	archivedDisabledKey, err := client.APIKey.Get(ctx, disabledPersonalKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusArchived, archivedDisabledKey.Status)

	userKey, err := client.APIKey.Get(ctx, targetUserKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, userKey.Status)

	serviceKey, err := client.APIKey.Get(ctx, targetServiceKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, serviceKey.Status)

	unrelatedKey, err := client.APIKey.Get(ctx, otherKey.ID)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusEnabled, unrelatedKey.Status)

	// A read immediately after deletion must not return the stale enabled cache entry.
	refreshedKey, err := apiKeyService.GetAPIKey(ctx, targetKey.Key)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusArchived, refreshedKey.Status)

	authService := &AuthService{APIKeyService: apiKeyService}
	_, err = authService.AuthenticateAPIKey(ctx, targetKey.Key)
	require.ErrorContains(t, err, "api key not enabled")
}
