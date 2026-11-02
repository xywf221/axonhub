package ent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPromptChannelRelationship(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:prompt-channel?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	prompt := client.Prompt.Create().
		SetName("prompt").
		SetRole("system").
		SetContent("content").
		SetSettings(objects.PromptSettings{}).
		SaveX(ctx)

	require.Equal(t, "prompt", prompt.Name)
}
