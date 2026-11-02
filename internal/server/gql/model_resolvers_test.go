package gql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestQueryModels_AuthenticatedUserCanReadModels(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:query-models-access?mode=memory&_fk=1")
	defer client.Close()

	setupCtx := authz.WithTestBypass(context.Background())
	userEntity := client.User.Create().
		SetEmail("query-models-user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SaveX(setupCtx)
	client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("query-models-channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"query-model"}).
		SetDefaultTestModel("query-model").
		SetStatus(channel.StatusEnabled).
		SaveX(setupCtx)

	ctx := authz.NewUserContext(context.Background(), userEntity.ID)
	ctx = contexts.WithUser(ctx, userEntity)

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	channelService := biz.NewChannelServiceForTest(client)
	modelService := biz.NewModelService(biz.ModelServiceParams{
		ChannelService: channelService,
		SystemService:  systemService,
		Ent:            client,
	})
	resolver := &queryResolver{&Resolver{
		client:         client,
		systemService:  systemService,
		channelService: channelService,
		modelService:   modelService,
	}}

	models, err := resolver.QueryModels(ctx, QueryModelsInput{})
	require.NoError(t, err)
	require.Contains(t, models, &biz.ModelIdentityWithStatus{ID: "query-model", Status: channel.StatusEnabled})
	channels, err := resolver.AllChannelSummarys(ctx, nil)
	require.NoError(t, err)
	require.Len(t, channels, 1)
}
