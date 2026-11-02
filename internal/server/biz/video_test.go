package biz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/openai"
	zenmuxtransformer "github.com/looplj/axonhub/llm/transformer/zenmux"
)

type videoServiceFixture struct {
	client  *ent.Client
	ctx     context.Context
	service *VideoService
	channel *ent.Channel
}

func newVideoServiceFixture(t *testing.T, channelType channel.Type, baseURL string) videoServiceFixture {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	channelCreate := client.Channel.Create().
		SetName(t.Name()).
		SetType(channelType).
		SetBaseURL(baseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"video-model"}).
		SetDefaultTestModel("video-model")
	if isZenmuxChannelType(channelType) {
		channelCreate.SetEndpoints([]objects.ChannelEndpoint{{APIFormat: llm.APIFormatZenmuxVideo.String()}})
	}
	channelEntity, err := channelCreate.Save(ctx)
	require.NoError(t, err)

	channelService := NewChannelServiceForTest(client)
	requestService := &RequestService{AbstractService: &AbstractService{db: client}}

	return videoServiceFixture{
		client:  client,
		ctx:     ctx,
		service: NewVideoService(channelService, requestService),
		channel: channelEntity,
	}
}

func (f videoServiceFixture) createTask(t *testing.T, format, externalID string) *ent.Request {
	return f.createTaskForAPIKey(t, 0, format, externalID)
}

func (f videoServiceFixture) createTaskForAPIKey(t *testing.T, apiKeyID int, format, externalID string) *ent.Request {
	t.Helper()

	create := f.client.Request.Create().
		SetModelID("video-model").
		SetFormat(format).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(request.StatusProcessing).
		SetChannelID(f.channel.ID).
		SetExternalID(externalID)
	if apiKeyID != 0 {
		create.SetAPIKeyID(apiKeyID)
	}

	task, err := create.Save(f.ctx)
	require.NoError(t, err)

	return task
}

func TestVideoService_ExternalIDLookupRespectsAPIKeyScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task","object":"video","status":"queued"}`))
	}))
	t.Cleanup(server.Close)

	fixture := newVideoServiceFixture(t, channel.TypeZenmux, server.URL)
	fixture.service.ChannelService.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	fixture.service.RequestService.SystemService = NewSystemService(SystemServiceParams{Ent: fixture.client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})

	keyOne, err := fixture.client.APIKey.Create().
		SetName("key-one").
		SetKey("key-one-secret").
		SetType(apikey.TypeServiceAccount).
		SetScopes([]string{string(scopes.ScopeWriteRequests)}).
		Save(fixture.ctx)
	require.NoError(t, err)
	keyTwo, err := fixture.client.APIKey.Create().
		SetName("key-two").
		SetKey("key-two-secret").
		SetType(apikey.TypeServiceAccount).
		SetScopes([]string{string(scopes.ScopeWriteRequests)}).
		Save(fixture.ctx)
	require.NoError(t, err)

	keyOneTask := fixture.createTaskForAPIKey(t, keyOne.ID, llm.APIFormatOpenAIVideo.String(), "key-one-task")
	keyTwoTask := fixture.createTaskForAPIKey(t, keyTwo.ID, llm.APIFormatOpenAIVideo.String(), "key-two-task")
	fixture.createExecution(t, keyOneTask, llm.APIFormatZenmuxVideo.String(), "key-one-task")
	fixture.createExecution(t, keyTwoTask, llm.APIFormatZenmuxVideo.String(), "key-two-task")

	keyOneCtx := contexts.WithAPIKey(fixture.ctx, keyOne)
	keyTwoCtx := contexts.WithAPIKey(fixture.ctx, keyTwo)

	_, err = fixture.service.GetTaskByExternalID(keyOneCtx, "key-one-task")
	require.NoError(t, err)

	_, err = fixture.service.GetTaskByExternalID(keyTwoCtx, "key-two-task")
	require.NoError(t, err)

	_, err = fixture.service.GetTaskByExternalID(keyOneCtx, "key-two-task")
	require.Error(t, err)
	_, err = fixture.service.GetTaskByExternalID(keyTwoCtx, "key-one-task")
	require.Error(t, err)
	require.Error(t, fixture.service.DeleteTaskByExternalID(keyOneCtx, "key-two-task"))
	require.Error(t, fixture.service.DeleteTaskByExternalID(keyTwoCtx, "key-one-task"))
}

func (f videoServiceFixture) createExecution(t *testing.T, task *ent.Request, format, externalID string) {
	t.Helper()

	_, err := f.client.RequestExecution.Create().
		SetRequestID(task.ID).
		SetChannelID(f.channel.ID).
		SetModelID("video-model").
		SetFormat(format).
		SetExternalID(externalID).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		Save(f.ctx)
	require.NoError(t, err)
}

func TestVideoService_LoadTask_UsesLatestAssociatedExecutionFormatAcrossRetries(t *testing.T) {
	fixture := newVideoServiceFixture(t, channel.TypeZenmux, "https://zenmux.example.invalid/api/v1")
	task := fixture.createTask(t, llm.APIFormatOpenAIVideo.String(), "native-task")
	fixture.createExecution(t, task, llm.APIFormatOpenAIVideo.String(), "openai-task")
	fixture.createExecution(t, task, llm.APIFormatZenmuxVideo.String(), "native-task")

	loaded, _, outbound, err := fixture.service.loadTask(fixture.ctx, task.ID)

	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIVideo.String(), loaded.Format, "public request format remains storage-compatible")
	require.IsType(t, &zenmuxtransformer.OutboundTransformer{}, outbound)
}

func TestVideoService_LoadTask_SkipsNewerExecutionWithoutProviderTaskAssociation(t *testing.T) {
	fixture := newVideoServiceFixture(t, channel.TypeZenmux, "https://zenmux.example.invalid/api/v1")
	task := fixture.createTask(t, llm.APIFormatOpenAIVideo.String(), "native-task")
	fixture.createExecution(t, task, llm.APIFormatZenmuxVideo.String(), "native-task")
	fixture.createExecution(t, task, llm.APIFormatOpenAIVideo.String(), "")

	_, _, outbound, err := fixture.service.loadTask(fixture.ctx, task.ID)

	require.NoError(t, err)
	require.IsType(t, &zenmuxtransformer.OutboundTransformer{}, outbound)
}

func TestVideoService_LoadTask_UsesPersistedOpenAIVideoFormatOnZenMuxChannel(t *testing.T) {
	fixture := newVideoServiceFixture(t, channel.TypeZenmux, "https://zenmux.example.invalid/api/v1")
	task := fixture.createTask(t, llm.APIFormatOpenAIVideo.String(), "openai-task")
	fixture.createExecution(t, task, llm.APIFormatZenmuxVideo.String(), "old-native-task")
	fixture.createExecution(t, task, llm.APIFormatOpenAIVideo.String(), "openai-task")

	_, _, outbound, err := fixture.service.loadTask(fixture.ctx, task.ID)

	require.NoError(t, err)
	require.IsType(t, &openai.OutboundTransformer{}, outbound)
}

func TestVideoService_LoadTask_RejectsAssociatedNonVideoExecutionFormat(t *testing.T) {
	fixture := newVideoServiceFixture(t, channel.TypeZenmux, "https://zenmux.example.invalid/api/v1")
	task := fixture.createTask(t, llm.APIFormatOpenAIVideo.String(), "native-task")
	fixture.createExecution(t, task, llm.APIFormatOpenAIChatCompletion.String(), "native-task")

	_, _, _, err := fixture.service.loadTask(fixture.ctx, task.ID)

	require.ErrorIs(t, err, ErrInternal)
	require.ErrorContains(t, err, "does not support video task operations")
}

func TestVideoService_LoadTask_UsesLegacyChannelFallbackWithoutPersistedExecutionFormat(t *testing.T) {
	tests := []struct {
		name          string
		channelType   channel.Type
		requestFormat llm.APIFormat
		wantOutbound  any
	}{
		{name: "ZenMux legacy OpenAI video", channelType: channel.TypeZenmux, requestFormat: llm.APIFormatOpenAIVideo, wantOutbound: &openai.OutboundTransformer{}},
		{name: "Doubao legacy Seedance video", channelType: channel.TypeDoubao, requestFormat: llm.APIFormatSeedanceVideo, wantOutbound: &doubao.OutboundTransformer{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newVideoServiceFixture(t, tt.channelType, "https://video.example.invalid")
			task := fixture.createTask(t, tt.requestFormat.String(), "legacy-task")
			fixture.createExecution(t, task, "", "legacy-task")

			_, _, outbound, err := fixture.service.loadTask(fixture.ctx, task.ID)

			require.NoError(t, err)
			require.IsType(t, tt.wantOutbound, outbound)
		})
	}
}

func TestVideoService_DeleteTask_CancelsZenMuxTaskLocallyWithoutUpstreamDelete(t *testing.T) {
	var upstreamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	fixture := newVideoServiceFixture(t, channel.TypeZenmux, server.URL)
	fixture.service.ChannelService.httpClient = httpclient.NewHttpClientWithClient(server.Client())
	task := fixture.createTask(t, llm.APIFormatOpenAIVideo.String(), "native-task")
	fixture.createExecution(t, task, llm.APIFormatZenmuxVideo.String(), "native-task")

	err := fixture.service.DeleteTask(fixture.ctx, task.ID)

	require.NoError(t, err)
	require.Zero(t, upstreamCalls.Load())
	updated, err := fixture.client.Request.Get(fixture.ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, updated.Status)
}
