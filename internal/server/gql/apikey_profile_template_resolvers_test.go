package gql

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
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func setupTestAPIKeyProfileTemplateResolvers(t *testing.T) (*mutationResolver, *queryResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	svc := biz.NewAPIKeyProfileTemplateService(biz.APIKeyProfileTemplateServiceParams{
		Ent: client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	resolver := &Resolver{
		client:                       client,
		apiKeyProfileTemplateService: svc,
	}

	return &mutationResolver{resolver}, &queryResolver{resolver}, ctx, client
}

func createTestUserWithScopes(t *testing.T, ctx context.Context, client *ent.Client) *ent.User {
	t.Helper()

	hashedPassword, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail(fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	return testUser
}

func TestApiKeyProfileTemplate_CreateTemplate(t *testing.T) {
	mutationResolver, _, ctx, client := setupTestAPIKeyProfileTemplateResolvers(t)
	defer client.Close()

	testUser := createTestUserWithScopes(t, ctx, client)
	ctx = contexts.WithUser(ctx, testUser)

	input := ent.CreateAPIKeyProfileTemplateInput{
		Name:        "my-template",
		Description: ptrStr("A test template"),
	}

	profile := objects.APIKeyProfile{
		Name: "test-profile",
		ModelMappings: []objects.ModelMapping{
			{From: "gpt-4", To: "gpt-4-turbo"},
		},
	}

	template, err := mutationResolver.CreateAPIKeyProfileTemplate(ctx, input, profile)
	require.NoError(t, err)
	require.NotNil(t, template)
	require.Equal(t, "my-template", template.Name)
	require.NotNil(t, template.Profile)
	require.Equal(t, "my-template", template.Profile.Name)
}

func TestApiKeyProfileTemplate_UpdateTemplate(t *testing.T) {
	mutationResolver, _, ctx, client := setupTestAPIKeyProfileTemplateResolvers(t)
	defer client.Close()

	testUser := createTestUserWithScopes(t, ctx, client)
	ctx = contexts.WithUser(ctx, testUser)

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("original-name").
		SetDescription("original desc").
		SetProfile(&objects.APIKeyProfile{Name: "original-profile"}).
		Save(ctx)
	require.NoError(t, err)

	input := ent.UpdateAPIKeyProfileTemplateInput{
		Name:        ptrStr("updated-name"),
		Description: ptrStr("updated desc"),
	}

	updatedProfile := &objects.APIKeyProfile{
		Name: "updated-profile",
		ModelMappings: []objects.ModelMapping{
			{From: "claude-3", To: "claude-3-opus"},
		},
	}

	result, err := mutationResolver.UpdateAPIKeyProfileTemplate(ctx, objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: template.ID}, input, updatedProfile)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "updated-name", result.Name)
	require.Equal(t, "updated desc", result.Description)
	require.Equal(t, "updated-name", result.Profile.Name)
}

func TestApiKeyProfileTemplate_DeleteTemplate(t *testing.T) {
	mutationResolver, _, ctx, client := setupTestAPIKeyProfileTemplateResolvers(t)
	defer client.Close()

	testUser := createTestUserWithScopes(t, ctx, client)
	ctx = contexts.WithUser(ctx, testUser)

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("to-delete").
		SetDescription("delete me").
		SetProfile(&objects.APIKeyProfile{Name: "delete-profile"}).
		Save(ctx)
	require.NoError(t, err)

	result, err := mutationResolver.DeleteAPIKeyProfileTemplate(ctx, objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: template.ID})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "to-delete", result.Name)

	_, err = client.APIKeyProfileTemplate.Get(ctx, template.ID)
	require.Error(t, err)
}

func TestApiKeyProfileTemplate_LoadTemplate(t *testing.T) {
	mutationResolver, _, ctx, client := setupTestAPIKeyProfileTemplateResolvers(t)
	defer client.Close()

	testUser := createTestUserWithScopes(t, ctx, client)
	ctx = contexts.WithUser(ctx, testUser)

	existingProfiles := &objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles: []objects.APIKeyProfile{
			{Name: "Default"},
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

	templateProfile := &objects.APIKeyProfile{
		ModelMappings: []objects.ModelMapping{
			{From: "gpt-4", To: "gpt-4-turbo"},
		},
	}

	template, err := client.APIKeyProfileTemplate.Create().
		SetName("load-template").
		SetDescription("load me").
		SetProfile(templateProfile).
		Save(ctx)
	require.NoError(t, err)

	input := LoadAPIKeyProfileTemplateInput{
		TemplateID: objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: template.ID},
		APIKeyID:   objects.GUID{Type: ent.TypeAPIKey, ID: apiKey.ID},
	}

	result, err := mutationResolver.LoadAPIKeyProfileTemplate(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Profiles)
	require.Len(t, result.Profiles.Profiles, 2)
	require.Equal(t, "Default", result.Profiles.Profiles[0].Name)
	require.Equal(t, "load-template", result.Profiles.Profiles[1].Name)
}

func TestApiKeyProfileTemplate_QueryTemplates(t *testing.T) {
	_, queryResolver, ctx, client := setupTestAPIKeyProfileTemplateResolvers(t)
	defer client.Close()

	testUser := createTestUserWithScopes(t, ctx, client)
	ctx = contexts.WithUser(ctx, testUser)

	for i := range 3 {
		_, err := client.APIKeyProfileTemplate.Create().
			SetName(fmt.Sprintf("template-%d", i)).
			SetDescription(fmt.Sprintf("template %d", i)).
			SetProfile(&objects.APIKeyProfile{Name: fmt.Sprintf("profile-%d", i)}).
			Save(ctx)
		require.NoError(t, err)
	}

	first := 10
	conn, err := queryResolver.APIKeyProfileTemplates(ctx, nil, &first, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, 3, conn.TotalCount)
}

func ptrStr(s string) *string {
	return &s
}
