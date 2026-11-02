package backup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/objects"
)

func TestBackupService_Backup_WithAPIKeys(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	require.NotNil(t, user)

	ak1 := createBackupTestAPIKey(t, client, ctx, user, "API Key 1", "sk-test-key-1")
	ak2 := createBackupTestAPIKey(t, client, ctx, user, "API Key 2", "sk-test-key-2")

	data, err := service.Backup(ctx, BackupOptions{
		IncludeAPIKeys: true,
	})
	require.NoError(t, err)
	require.NotNil(t, data)

	var backupData BackupData

	err = json.Unmarshal(data, &backupData)
	require.NoError(t, err)

	require.Equal(t, BackupVersion, backupData.Version)
	require.Len(t, backupData.APIKeys, 2)

	require.Equal(t, ak1.Name, backupData.APIKeys[0].Name)
	require.Equal(t, ak1.Key, backupData.APIKeys[0].Key)

	require.Equal(t, ak2.Name, backupData.APIKeys[1].Name)
	require.Equal(t, ak2.Key, backupData.APIKeys[1].Key)
}

func TestBackupService_Restore_APIKeys_NewKeys(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-new-key-1",
					Name:     "New API Key 1",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
			},
			{
				APIKey: ent.APIKey{
					Key:      "sk-new-key-2",
					Name:     "New API Key 2",
					Type:     "user",
					Status:   "enabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	apiKeys, err := client.APIKey.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, apiKeys, 2)

	ak1, err := client.APIKey.Query().
		Where(apikey.Key("sk-new-key-1")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New API Key 1", ak1.Name)

	ak2, err := client.APIKey.Query().
		Where(apikey.Key("sk-new-key-2")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New API Key 2", ak2.Name)
}

func TestBackupService_Restore_APIKeys_ConflictSkip(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	createBackupTestAPIKey(t, client, ctx, user, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat", "embedding"},
					Profiles: profiles,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategySkip,
	})
	require.NoError(t, err)

	ak, err := client.APIKey.Query().
		Where(apikey.Key("sk-existing-key")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Existing Key", ak.Name)
	require.Equal(t, apikey.TypeUser, ak.Type)
	require.Equal(t, apikey.StatusEnabled, ak.Status)
}

func TestBackupService_Restore_APIKeys_ConflictOverwrite(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	createBackupTestAPIKey(t, client, ctx, user, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "production",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "production",
				ModelIDs: []string{"gpt-4", "claude-3"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat", "embedding"},
					Profiles: profiles,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	ak, err := client.APIKey.Query().
		Where(apikey.Key("sk-existing-key")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Updated Key Name", ak.Name)
	require.Equal(t, apikey.TypeServiceAccount, ak.Type)
	require.Equal(t, apikey.StatusDisabled, ak.Status)
	require.Equal(t, []string{"chat", "embedding"}, ak.Scopes)
	require.Equal(t, "production", ak.Profiles.ActiveProfile)
}

func TestBackupService_Restore_APIKeys_ConflictError(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	createBackupTestAPIKey(t, client, ctx, user, "Existing Key", "sk-existing-key")

	profiles := &objects.APIKeyProfiles{
		ActiveProfile: "default",
		Profiles: []objects.APIKeyProfile{
			{
				Name:     "default",
				ModelIDs: []string{"gpt-4"},
			},
		},
	}

	backupData := BackupData{
		Version: BackupVersion,
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:      "sk-existing-key",
					Name:     "Updated Key Name",
					Type:     "service_account",
					Status:   "disabled",
					Scopes:   []string{"chat"},
					Profiles: profiles,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeAPIKeys:         true,
		APIKeyConflictStrategy: ConflictStrategyError,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}
