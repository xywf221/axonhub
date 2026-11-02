package gql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func setupTestQueryResolver(t *testing.T) (*queryResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	resolver := &queryResolver{&Resolver{client: client}}

	return resolver, ctx, client
}

func TestQueryResolver_AllChannelSummarys_RequiresChannelReadScope(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Protected").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"protected-model"}).
		SetDefaultTestModel("protected-model").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	unauthorizedCtx := ent.NewContext(context.Background(), client)
	_, err = resolver.AllChannelSummarys(unauthorizedCtx, nil)
	require.Error(t, err)
}
