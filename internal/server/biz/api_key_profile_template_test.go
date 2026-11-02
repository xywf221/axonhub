package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/apikeyprofiletemplate"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func setupTestTemplateService(t *testing.T) (*APIKeyProfileTemplateService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	svc := NewAPIKeyProfileTemplateService(APIKeyProfileTemplateServiceParams{
		Ent: client,
	})

	return svc, client
}

func TestAPIKeyProfileTemplate(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)
	_ = testUser

	// Test 1: Create ApiKeyProfileTemplate with name, description, and profile JSON
	t.Run("Create ApiKeyProfileTemplate", func(t *testing.T) {
		profile := &objects.APIKeyProfile{
			Name: "test-profile",
			ModelMappings: []objects.ModelMapping{
				{From: "gpt-4", To: "gpt-4-turbo"},
			},
		}

		template, err := client.APIKeyProfileTemplate.Create().
			SetName("my-template").
			SetDescription("A test template").
			SetProfile(profile).
			Save(ctx)
		require.NoError(t, err)
		require.NotNil(t, template)
		require.Equal(t, "my-template", template.Name)
		require.Equal(t, "A test template", template.Description)

		// Verify profile was stored correctly
		require.NotNil(t, template.Profile)
		require.Equal(t, "test-profile", template.Profile.Name)
	})

	// Test 2: Query back the created template
	t.Run("Query ApiKeyProfileTemplate", func(t *testing.T) {
		template, err := client.APIKeyProfileTemplate.Query().
			Where(apikeyprofiletemplate.Name("my-template")).
			Only(ctx)
		require.NoError(t, err)
		require.NotNil(t, template)
		require.Equal(t, "A test template", template.Description)
	})

	// Test 3: Unique index - duplicate name should fail
	t.Run("Unique index on name", func(t *testing.T) {
		// Try to create duplicate
		_, err := client.APIKeyProfileTemplate.Create().
			SetName("my-template").
			SetDescription("Duplicate").
			SetProfile(&objects.APIKeyProfile{Name: "dup"}).
			Save(ctx)
		require.Error(t, err)
	})
}

// TestLoadTemplate_HappyPath tests loading a template into an API key with different profile names.
// Profile appended, existing profiles unchanged, active profile unchanged.
func TestLoadTemplate_HappyPath(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create API key with existing profiles
	existingProfiles := &objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles: []objects.APIKeyProfile{
			{
				Name: "Default",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4", To: "gpt-4-turbo"},
				},
			},
		},
	}

	apiKey, err := client.APIKey.Create().
		SetName("test-api-key").
		SetKey(fmt.Sprintf("ah-test-%d", time.Now().UnixNano())).
		SetUserID(testUser.ID).
		SetType(apikey.TypeUser).
		SetProfiles(existingProfiles).
		Save(ctx)
	require.NoError(t, err)

	// Create template with a different profile name
	templateProfile := &objects.APIKeyProfile{
		Name: "Production",
		ModelMappings: []objects.ModelMapping{
			{From: "claude-3", To: "claude-3-opus"},
		},
	}

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProfile(templateProfile).
		Save(ctx)
	require.NoError(t, err)

	// Load template into API key
	updatedKey, err := svc.LoadTemplate(ctx, template.ID, apiKey.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedKey)

	// Verify profiles
	require.NotNil(t, updatedKey.Profiles)
	require.Equal(t, "Default", updatedKey.Profiles.ActiveProfile, "active profile should not change")
	require.Len(t, updatedKey.Profiles.Profiles, 2, "should have 2 profiles (original + loaded)")

	// Verify original profile is unchanged
	require.Equal(t, "Default", updatedKey.Profiles.Profiles[0].Name)
	require.Len(t, updatedKey.Profiles.Profiles[0].ModelMappings, 1)
	require.Equal(t, "gpt-4", updatedKey.Profiles.Profiles[0].ModelMappings[0].From)

	// Verify loaded profile is appended
	require.Equal(t, "Production", updatedKey.Profiles.Profiles[1].Name)
	require.NotNil(t, updatedKey.Profiles.Profiles[1].TemplateID)
	require.Equal(t, template.ID, *updatedKey.Profiles.Profiles[1].TemplateID)
	require.Equal(t, template.Name, updatedKey.Profiles.Profiles[1].TemplateName)
	require.Len(t, updatedKey.Profiles.Profiles[1].ModelMappings, 1)
	require.Equal(t, "claude-3", updatedKey.Profiles.Profiles[1].ModelMappings[0].From)
}

func TestUpdateTemplatePublishesToLinkedAndUnchangedLegacyProfiles(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("Production").
		SetProfile(&objects.APIKeyProfile{
			Name:          "Production",
			ModelMappings: []objects.ModelMapping{{From: "claude", To: "old-model"}},
		}).
		Save(ctx)
	require.NoError(t, err)

	templateID := template.ID
	createKey := func(name string, profile objects.APIKeyProfile) *ent.APIKey {
		key, createErr := client.APIKey.Create().
			SetName(name).
			SetKey(fmt.Sprintf("ah-%s-%d", name, time.Now().UnixNano())).
			SetType(apikey.TypeUser).
			SetProfiles(&objects.APIKeyProfiles{ActiveProfile: profile.Name, Profiles: []objects.APIKeyProfile{profile}}).
			Save(ctx)
		require.NoError(t, createErr)
		return key
	}

	linkedKey := createKey("linked", objects.APIKeyProfile{
		Name:          "Local alias",
		TemplateID:    &templateID,
		TemplateName:  template.Name,
		ModelMappings: []objects.ModelMapping{{From: "claude", To: "old-model"}},
	})
	legacyKey := createKey("legacy", objects.APIKeyProfile{
		Name:          "Production",
		ModelMappings: []objects.ModelMapping{{From: "claude", To: "old-model"}},
	})
	detachedKey := createKey("detached", objects.APIKeyProfile{
		Name:          "Production",
		ModelMappings: []objects.ModelMapping{{From: "claude", To: "custom-model"}},
	})

	count, err := svc.CountLinkedProfiles(ctx, template)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	_, err = svc.UpdateTemplate(ctx, template.ID, ent.UpdateAPIKeyProfileTemplateInput{}, &objects.APIKeyProfile{
		Name:          "Production",
		ModelMappings: []objects.ModelMapping{{From: "claude", To: "new-model"}},
	})
	require.NoError(t, err)

	for _, key := range []*ent.APIKey{linkedKey, legacyKey} {
		updated, getErr := client.APIKey.Get(ctx, key.ID)
		require.NoError(t, getErr)
		require.Equal(t, "new-model", updated.Profiles.Profiles[0].ModelMappings[0].To)
		require.NotNil(t, updated.Profiles.Profiles[0].TemplateID)
		require.Equal(t, template.ID, *updated.Profiles.Profiles[0].TemplateID)
	}

	updatedLinked, err := client.APIKey.Get(ctx, linkedKey.ID)
	require.NoError(t, err)
	require.Equal(t, "Local alias", updatedLinked.Profiles.Profiles[0].Name)

	updatedDetached, err := client.APIKey.Get(ctx, detachedKey.ID)
	require.NoError(t, err)
	require.Equal(t, "custom-model", updatedDetached.Profiles.Profiles[0].ModelMappings[0].To)
	require.Nil(t, updatedDetached.Profiles.Profiles[0].TemplateID)

	_, err = svc.DeleteTemplate(ctx, template.ID)
	require.NoError(t, err)
	for _, key := range []*ent.APIKey{linkedKey, legacyKey} {
		updated, getErr := client.APIKey.Get(ctx, key.ID)
		require.NoError(t, getErr)
		require.Nil(t, updated.Profiles.Profiles[0].TemplateID)
		require.Empty(t, updated.Profiles.Profiles[0].TemplateName)
		require.Equal(t, "new-model", updated.Profiles.Profiles[0].ModelMappings[0].To)
	}
}

func TestDetachModifiedTemplateProfiles(t *testing.T) {
	templateID := 7
	existing := &objects.APIKeyProfiles{Profiles: []objects.APIKeyProfile{
		{
			Name:          "linked",
			TemplateID:    &templateID,
			TemplateName:  "Production",
			ModelMappings: []objects.ModelMapping{{From: "claude", To: "old-model"}},
		},
	}}

	unchanged := &objects.APIKeyProfiles{Profiles: []objects.APIKeyProfile{
		{
			Name:          "linked",
			TemplateID:    &templateID,
			TemplateName:  "tampered name",
			ModelMappings: []objects.ModelMapping{{From: "claude", To: "old-model"}},
		},
	}}
	detachModifiedTemplateProfiles(existing, unchanged)
	require.NotNil(t, unchanged.Profiles[0].TemplateID)
	require.Equal(t, "Production", unchanged.Profiles[0].TemplateName)

	modified := &objects.APIKeyProfiles{Profiles: []objects.APIKeyProfile{
		{
			Name:          "linked",
			TemplateID:    &templateID,
			TemplateName:  "Production",
			ModelMappings: []objects.ModelMapping{{From: "claude", To: "custom-model"}},
		},
	}}
	detachModifiedTemplateProfiles(existing, modified)
	require.Nil(t, modified.Profiles[0].TemplateID)
	require.Empty(t, modified.Profiles[0].TemplateName)
}

// TestLoadTemplate_NameConflict tests loading a template where profile name already exists.
// Auto-rename with suffix " (1)".
func TestLoadTemplate_NameConflict(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create API key with a profile named "Production"
	existingProfiles := &objects.APIKeyProfiles{
		ActiveProfile: "Production",
		Profiles: []objects.APIKeyProfile{
			{
				Name: "Production",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4", To: "gpt-4-turbo"},
				},
			},
		},
	}

	apiKey, err := client.APIKey.Create().
		SetName("test-api-key").
		SetKey(fmt.Sprintf("ah-test-%d", time.Now().UnixNano())).
		SetUserID(testUser.ID).
		SetType(apikey.TypeUser).
		SetProfiles(existingProfiles).
		Save(ctx)
	require.NoError(t, err)

	// Create template also named "Production"
	templateProfile := &objects.APIKeyProfile{
		Name: "Production",
		ModelMappings: []objects.ModelMapping{
			{From: "claude-3", To: "claude-3-opus"},
		},
	}

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProfile(templateProfile).
		Save(ctx)
	require.NoError(t, err)

	// Load template into API key
	updatedKey, err := svc.LoadTemplate(ctx, template.ID, apiKey.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedKey)

	// Verify profiles
	require.NotNil(t, updatedKey.Profiles)
	require.Equal(t, "Production", updatedKey.Profiles.ActiveProfile, "active profile should not change")
	require.Len(t, updatedKey.Profiles.Profiles, 2, "should have 2 profiles")

	// Original profile unchanged
	require.Equal(t, "Production", updatedKey.Profiles.Profiles[0].Name)

	// Loaded profile should be auto-renamed to "Production (1)"
	require.Equal(t, "Production (1)", updatedKey.Profiles.Profiles[1].Name)
	require.Len(t, updatedKey.Profiles.Profiles[1].ModelMappings, 1)
	require.Equal(t, "claude-3", updatedKey.Profiles.Profiles[1].ModelMappings[0].From)
}

// TestLoadTemplate_MultipleConflicts tests loading a template when multiple name conflicts exist.
// Key has "Production", "Production (1)", template is "Production". Should become "Production (2)".
func TestLoadTemplate_MultipleConflicts(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create API key with "Production" and "Production (1)"
	existingProfiles := &objects.APIKeyProfiles{
		ActiveProfile: "Production",
		Profiles: []objects.APIKeyProfile{
			{
				Name: "Production",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4", To: "gpt-4-turbo"},
				},
			},
			{
				Name: "Production (1)",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-3.5", To: "gpt-3.5-turbo"},
				},
			},
		},
	}

	apiKey, err := client.APIKey.Create().
		SetName("test-api-key").
		SetKey(fmt.Sprintf("ah-test-%d", time.Now().UnixNano())).
		SetUserID(testUser.ID).
		SetType(apikey.TypeUser).
		SetProfiles(existingProfiles).
		Save(ctx)
	require.NoError(t, err)

	// Create template named "Production"
	templateProfile := &objects.APIKeyProfile{
		Name: "Production",
		ModelMappings: []objects.ModelMapping{
			{From: "claude-3", To: "claude-3-opus"},
		},
	}

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProfile(templateProfile).
		Save(ctx)
	require.NoError(t, err)

	// Load template into API key
	updatedKey, err := svc.LoadTemplate(ctx, template.ID, apiKey.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedKey)

	// Verify profiles
	require.NotNil(t, updatedKey.Profiles)
	require.Len(t, updatedKey.Profiles.Profiles, 3, "should have 3 profiles")

	// Original profiles unchanged
	require.Equal(t, "Production", updatedKey.Profiles.Profiles[0].Name)
	require.Equal(t, "Production (1)", updatedKey.Profiles.Profiles[1].Name)

	// Loaded profile should be auto-renamed to "Production (2)"
	require.Equal(t, "Production (2)", updatedKey.Profiles.Profiles[2].Name)
}

// TestLoadTemplate_TemplateNotFound tests that loading a non-existent template returns an error.
func TestLoadTemplate_TemplateNotFound(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create test user
	hashedPassword, err := HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(ctx)
	require.NoError(t, err)

	// Create API key
	apiKey, err := client.APIKey.Create().
		SetName("test-api-key").
		SetKey(fmt.Sprintf("ah-test-%d", time.Now().UnixNano())).
		SetUserID(testUser.ID).
		SetType(apikey.TypeUser).
		Save(ctx)
	require.NoError(t, err)

	// Try to load non-existent template
	_, err = svc.LoadTemplate(ctx, 99999, apiKey.ID)
	require.Error(t, err)
}

// TestLoadTemplate_APIKeyNotFound tests that loading into a non-existent API key returns an error.
func TestLoadTemplate_APIKeyNotFound(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create template
	templateProfile := &objects.APIKeyProfile{
		Name: "Production",
	}

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProfile(templateProfile).
		Save(ctx)
	require.NoError(t, err)

	// Try to load into non-existent API key
	_, err = svc.LoadTemplate(ctx, template.ID, 99999)
	require.Error(t, err)
}

// TestLoadTemplate_PersonalKeyGuard verifies that only the creator of a
// personal API key or a system owner can load a template into it.
func TestLoadTemplate_PersonalKeyGuard(t *testing.T) {
	svc, client := setupTestTemplateService(t)
	defer client.Close()

	setupCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	newUser := func(name string, isSystemOwner bool) *ent.User {
		u, err := client.User.Create().
			SetEmail(fmt.Sprintf("%s-%d@example.com", name, time.Now().UnixNano())).
			SetPassword("password").
			SetIsOwner(isSystemOwner).
			SetStatus(user.StatusActivated).
			Save(setupCtx)
		require.NoError(t, err)

		return u
	}

	creator := newUser("creator", false)
	otherUser := newUser("other-user", false)
	systemOwner := newUser("system-owner", true)

	apiKey, err := client.APIKey.Create().
		SetName("personal-api-key").
		SetKey(fmt.Sprintf("ah-test-%d", time.Now().UnixNano())).
		SetUserID(creator.ID).
		SetType(apikey.TypePersonal).
		Save(setupCtx)
	require.NoError(t, err)

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProfile(&objects.APIKeyProfile{Name: "Production"}).
		Save(setupCtx)
	require.NoError(t, err)

	userCtx := func(u *ent.User) context.Context {
		return contexts.WithUser(setupCtx, u)
	}

	// Another regular user cannot load a template into someone else's personal key.
	_, err = svc.LoadTemplate(userCtx(otherUser), template.ID, apiKey.ID)
	require.ErrorContains(t, err, "personal API key can only be modified by its creator or a system owner")
	unchanged, err := client.APIKey.Get(setupCtx, apiKey.ID)
	require.NoError(t, err)
	require.True(t, unchanged.Profiles == nil || len(unchanged.Profiles.Profiles) == 0)

	// The creator can.
	updatedKey, err := svc.LoadTemplate(userCtx(creator), template.ID, apiKey.ID)
	require.NoError(t, err)
	require.Len(t, updatedKey.Profiles.Profiles, 1)

	// A system owner can too.
	updatedKey, err = svc.LoadTemplate(userCtx(systemOwner), template.ID, apiKey.ID)
	require.NoError(t, err)
	require.Len(t, updatedKey.Profiles.Profiles, 2)
}
