package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
)

const (
	webSearchToolTypePrefix = "web_search"
	webSearchQueryPrefix    = "Perform a web search for the query:"
	webSearchTimeout        = 30 * time.Second
	webSearchMaxBodyBytes   = 4 << 20
)

type webSearchTool struct {
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

type webSearchMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type webSearchRequest struct {
	Model    string             `json:"model"`
	Stream   bool               `json:"stream"`
	Messages []webSearchMessage `json:"messages"`
	Tools    []webSearchTool    `json:"tools"`
}

type webSearchUpstreamRequest struct {
	Query          string   `json:"query"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	BlockedDomains []string `json:"blocked_domains,omitempty"`
}

type webSearchUpstreamResult struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	PublishedTime string `json:"published_time"`
}

type webSearchUpstreamResponse struct {
	Results []webSearchUpstreamResult `json:"results"`
}

// tryServeNativeWebSearch answers Anthropic native web_search tool calls from the
// globally configured search endpoint. It reports whether the request was handled.
func (handlers *AnthropicHandlers) tryServeNativeWebSearch(c *gin.Context) bool {
	ctx := c.Request.Context()

	settings := handlers.SystemService.WebSearchSettingsOrDefault(
		authz.WithSystemBypass(ctx, "anthropic-native-web-search"),
	)
	if !settings.Enabled || settings.Endpoint == "" {
		return false
	}

	if c.Request.Body == nil {
		return false
	}

	body, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	if err != nil {
		log.Warn(ctx, "failed to read web search request body", log.Cause(err))

		return false
	}

	var req webSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}

	tool, ok := findWebSearchTool(req.Tools)
	if !ok {
		return false
	}

	query := extractWebSearchQuery(req.Messages)
	if query == "" {
		return false
	}

	results, err := callWebSearchEndpoint(ctx, settings.Endpoint, settings.APIKey, webSearchUpstreamRequest{
		Query:          query,
		AllowedDomains: tool.AllowedDomains,
		BlockedDomains: tool.BlockedDomains,
	})

	if err != nil {
		log.Warn(ctx, "web search endpoint failed",
			log.String("query", query),
			log.Cause(err),
		)
	}

	log.Info(ctx, "served web search",
		log.String("query", query),
		log.Int("results", len(results)),
		log.Bool("error", err != nil),
	)

	content := buildWebSearchContent(newWebSearchID("srvtoolu_"), query, results, err)
	messageID := newWebSearchID("msg_")

	if req.Stream {
		writeWebSearchStream(c, messageID, req.Model, content)
	} else {
		c.JSON(http.StatusOK, map[string]any{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"content":       content,
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		})
	}

	return true
}

func findWebSearchTool(tools []webSearchTool) (webSearchTool, bool) {
	for _, tool := range tools {
		if strings.HasPrefix(tool.Type, webSearchToolTypePrefix) {
			return tool, true
		}
	}

	return webSearchTool{}, false
}

func extractWebSearchQuery(messages []webSearchMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}

		text := strings.TrimSpace(messageText(messages[i].Content))
		if text == "" {
			continue
		}

		if idx := strings.Index(text, webSearchQueryPrefix); idx >= 0 {
			text = strings.TrimSpace(text[idx+len(webSearchQueryPrefix):])
		}

		if text != "" {
			return text
		}
	}

	return ""
}

func messageText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var builder strings.Builder

		for _, block := range blocks {
			if block.Type == "text" {
				builder.WriteString(block.Text)
			}
		}

		return builder.String()
	}

	return ""
}

func callWebSearchEndpoint(
	ctx context.Context,
	endpoint string,
	apiKey string,
	payload webSearchUpstreamRequest,
) ([]webSearchUpstreamResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal web search request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, webSearchTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build web search request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call web search endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, webSearchMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read web search response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("web search endpoint returned %d: %s", resp.StatusCode, truncate(string(respBody), 256))
	}

	var parsed webSearchUpstreamResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode web search response: %w", err)
	}

	return parsed.Results, nil
}

func buildWebSearchContent(toolUseID, query string, results []webSearchUpstreamResult, searchErr error) []map[string]any {
	var resultContent any

	if searchErr != nil {
		resultContent = map[string]any{
			"type":       "web_search_tool_result_error",
			"error_code": "unavailable",
		}
	} else {
		items := make([]map[string]any, 0, len(results))
		for _, result := range results {
			item := map[string]any{
				"type":  "web_search_result",
				"url":   result.URL,
				"title": result.Title,
			}
			if result.PublishedTime != "" {
				item["page_age"] = result.PublishedTime
			}

			items = append(items, item)
		}

		resultContent = items
	}

	return []map[string]any{
		{
			"type":  "server_tool_use",
			"id":    toolUseID,
			"name":  "web_search",
			"input": map[string]any{"query": query},
		},
		{
			"type":        "web_search_tool_result",
			"tool_use_id": toolUseID,
			"content":     resultContent,
		},
	}
}

func writeWebSearchStream(c *gin.Context, messageID, model string, content []map[string]any) {
	ctx := c.Request.Context()
	writer := c.Writer

	setSSEResponseHeaders(writer.Header())

	toolUseID, _ := content[0]["id"].(string)

	events := []struct {
		name string
		data any
	}{
		{
			name: "message_start",
			data: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":            messageID,
					"type":          "message",
					"role":          "assistant",
					"model":         model,
					"content":       []any{},
					"stop_reason":   nil,
					"stop_sequence": nil,
					"usage": map[string]int{
						"input_tokens":  0,
						"output_tokens": 0,
					},
				},
			},
		},
		{
			name: "content_block_start",
			data: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":  "server_tool_use",
					"id":    toolUseID,
					"name":  "web_search",
					"input": map[string]any{},
				},
			},
		},
		{
			name: "content_block_delta",
			data: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": mustMarshalJSON(content[0]["input"]),
				},
			},
		},
		{
			name: "content_block_stop",
			data: map[string]any{
				"type":  "content_block_stop",
				"index": 0,
			},
		},
		{
			name: "content_block_start",
			data: map[string]any{
				"type":          "content_block_start",
				"index":         1,
				"content_block": content[1],
			},
		},
		{
			name: "content_block_stop",
			data: map[string]any{
				"type":  "content_block_stop",
				"index": 1,
			},
		},
		{
			name: "message_delta",
			data: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason":   "end_turn",
					"stop_sequence": nil,
				},
				"usage": map[string]int{
					"output_tokens": 0,
				},
			},
		},
		{
			name: "message_stop",
			data: map[string]any{
				"type": "message_stop",
			},
		},
	}

	for _, event := range events {
		if err := writeSSEEvent(ctx, writer, event.name, event.data); err != nil {
			return
		}
	}
}

func mustMarshalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}

	return string(data)
}

func newWebSearchID(prefix string) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return prefix + fmt.Sprintf("%d", time.Now().UnixNano())
	}

	return prefix + hex.EncodeToString(buf)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return value[:limit]
}
