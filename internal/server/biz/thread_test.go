package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestThreadService(t *testing.T) (*ThreadService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	threadService := NewThreadService(
		client,
		NewTraceService(TraceServiceParams{
			RequestService: NewRequestService(
				client,
				systemService.CacheConfig,
				systemService,
				NewUsageLogService(client, systemService, NewChannelServiceForTest(client)),
				NewDataStorageService(
					DataStorageServiceParams{
						SystemService: systemService,
						CacheConfig:   xcache.Config{},
						Client:        client,
					},
				),
				NewLiveStreamRegistry(),
			),
			Ent: client,
		}),
	)

	return threadService, client
}

func TestThreadService_GetOrCreateThread(t *testing.T) {
	threadService, client := setupTestThreadService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	threadID := "thread-test-123"

	// Test creating a new thread
	thread1, err := threadService.GetOrCreateThread(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, thread1)
	require.Equal(t, threadID, thread1.ThreadID)

	// Test getting existing thread (should return the same thread)
	thread2, err := threadService.GetOrCreateThread(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, thread2)
	require.Equal(t, thread1.ID, thread2.ID)
	require.Equal(t, threadID, thread2.ThreadID)

	// Test creating a thread with different threadID
	differentThreadID := "thread-test-456"
	thread3, err := threadService.GetOrCreateThread(ctx, differentThreadID)
	require.NoError(t, err)
	require.NotNil(t, thread3)
	require.NotEqual(t, thread1.ID, thread3.ID)
	require.Equal(t, differentThreadID, thread3.ThreadID)
}

func TestThreadService_GetThreadByID(t *testing.T) {
	threadService, client := setupTestThreadService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	threadID := "thread-get-test-123"

	// Create a thread first
	createdThread, err := client.Thread.Create().
		SetThreadID(threadID).
		Save(ctx)
	require.NoError(t, err)

	// Test getting the thread
	retrievedThread, err := threadService.GetThreadByID(ctx, threadID)
	require.NoError(t, err)
	require.NotNil(t, retrievedThread)
	require.Equal(t, createdThread.ID, retrievedThread.ID)
	require.Equal(t, threadID, retrievedThread.ThreadID)

	// Test getting non-existent thread
	_, err = threadService.GetThreadByID(ctx, "non-existent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get thread")
}

func TestThreadService_GetOrCreateThread_NoClient(t *testing.T) {
	threadService, client := setupTestThreadService(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client) // Add client to context
	ctx = authz.WithTestBypass(ctx)

	// Test with ent client in context - should work
	thread, err := threadService.GetOrCreateThread(ctx, "thread-123")
	require.NoError(t, err)
	require.NotNil(t, thread)
	require.Equal(t, "thread-123", thread.ThreadID)
}
