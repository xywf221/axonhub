package biz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestSystemService_Initialize(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := ent.NewContext(t.Context(), client)

	migrator := datamigrate.NewMigrator(client)
	err := migrator.Run(ctx)
	require.NoError(t, err)

	service := biz.NewSystemService(biz.SystemServiceParams{})

	// Test system initialization with auto-generated secret key
	err = service.Initialize(ctx, &biz.InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Verify system is initialized
	isInitialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	require.True(t, isInitialized)

	// Verify secret key is set
	ctx = authz.WithTestBypass(ctx)
	secretKey, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Len(t, secretKey, 64) // Should be 64 hex characters (32 bytes)

	// Verify owner user is created
	owner, err := client.User.Query().Where().First(ctx)
	require.NoError(t, err)
	require.Equal(t, "owner@example.com", owner.Email)
	require.True(t, owner.IsOwner)

	// Test idempotency - calling Initialize again should not error
	// but should not change the existing secret key
	originalKey := secretKey
	err = service.Initialize(ctx, &biz.InitializeSystemParams{
		OwnerEmail:     "owner@example.com",
		OwnerPassword:  "password123",
		OwnerFirstName: "System",
		OwnerLastName:  "Owner",
		BrandName:      "Test Brand",
	})
	require.NoError(t, err)

	// Secret key should remain the same after second initialization
	secretKey2, err := service.SecretKey(ctx)
	require.NoError(t, err)
	require.Equal(t, originalKey, secretKey2)

	version, err := service.Version(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, version)

	// Verify primary data storage was created during initialization
	primaryDS, err := client.DataStorage.Query().
		Where().
		First(ctx)
	require.NoError(t, err)
	require.True(t, primaryDS.Primary)
	require.Equal(t, "Primary", primaryDS.Name)

	// Verify default data storage ID is set
	defaultID, err := service.DefaultDataStorageID(ctx)
	require.NoError(t, err)
	require.Equal(t, primaryDS.ID, defaultID)
}
