package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractWebSearchQuery(t *testing.T) {
	messages := []webSearchMessage{
		{Role: "user", Content: json.RawMessage(`"Perform a web search for the query: latest go release"`)},
	}

	assert.Equal(t, "latest go release", extractWebSearchQuery(messages))

	blocks := []webSearchMessage{
		{Role: "assistant", Content: json.RawMessage(`"ignored"`)},
		{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"Perform a web search for the query: axonhub web search"}]`)},
	}

	assert.Equal(t, "axonhub web search", extractWebSearchQuery(blocks))
	assert.Equal(t, "", extractWebSearchQuery([]webSearchMessage{{Role: "assistant", Content: json.RawMessage(`"hello"`)}}))
}

func TestFindWebSearchTool(t *testing.T) {
	tool, ok := findWebSearchTool([]webSearchTool{
		{Type: "function", Name: "Read"},
		{Type: "web_search_20250305", Name: "web_search"},
	})

	require.True(t, ok)
	assert.Equal(t, "web_search_20250305", tool.Type)

	_, ok = findWebSearchTool([]webSearchTool{{Type: "function", Name: "Read"}})
	assert.False(t, ok)
}

func TestBuildWebSearchContent(t *testing.T) {
	content := buildWebSearchContent("srvtoolu_1", "query", []webSearchUpstreamResult{
		{Title: "Example", URL: "https://example.com", PublishedTime: "2026-01-01"},
	}, nil)

	require.Len(t, content, 2)
	assert.Equal(t, "server_tool_use", content[0]["type"])
	assert.Equal(t, map[string]any{"query": "query"}, content[0]["input"])

	results, ok := content[1]["content"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, results, 1)
	assert.Equal(t, "https://example.com", results[0]["url"])
	assert.Equal(t, "2026-01-01", results[0]["page_age"])

	failure := buildWebSearchContent("srvtoolu_2", "query", nil, assert.AnError)
	assert.Equal(t, map[string]any{
		"type":       "web_search_tool_result_error",
		"error_code": "unavailable",
	}, failure[1]["content"])
}

func TestCallWebSearchEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.JSONEq(t, `{"query":"q","allowed_domains":["a.com"]}`, string(mustReadAll(t, r.Body)))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"T","url":"https://a.com","published_time":"2026-01-01"}]}`))
	}))
	defer server.Close()

	results, err := callWebSearchEndpoint(context.Background(), server.URL, "secret", webSearchUpstreamRequest{
		Query:          "q",
		AllowedDomains: []string{"a.com"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "https://a.com", results[0].URL)
}

func TestCallWebSearchEndpointErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"web_search_failed"}`))
	}))
	defer server.Close()

	_, err := callWebSearchEndpoint(context.Background(), server.URL, "", webSearchUpstreamRequest{Query: "q"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestWriteWebSearchStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request = c.Request.WithContext(context.Background())

	content := buildWebSearchContent("srvtoolu_1", "query", []webSearchUpstreamResult{{Title: "T", URL: "https://a.com"}}, nil)
	writeWebSearchStream(c, "msg_1", "claude-test", content)

	body := recorder.Body.String()

	for _, event := range []string{
		"event:message_start",
		"event:content_block_start",
		"event:content_block_delta",
		"event:content_block_stop",
		"event:message_delta",
		"event:message_stop",
		`"partial_json":"{\"query\":\"query\"}"`,
		`"type":"web_search_tool_result"`,
	} {
		assert.Contains(t, body, event)
	}
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
}

func mustReadAll(t *testing.T, reader io.Reader) []byte {
	t.Helper()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	return data
}
