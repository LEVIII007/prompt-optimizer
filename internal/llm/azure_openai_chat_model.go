package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// AzureOpenAIChatModel is a minimal ToolCallingChatModel implementation for Azure OpenAI Chat Completions.
//
// Endpoint example: https://{resource}.openai.azure.com
// Deployment example: gpt-4o-mini
// API version example: 2025-01-01-preview
type AzureOpenAIChatModel struct {
	endpoint   string
	apiKey     string
	apiVersion string
	deployment string

	temperature *float32
	maxTokens   *int

	tools []*schema.ToolInfo

	client *http.Client
}

var azureDefaultClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

func NewAzureOpenAIChatModel(endpoint, apiKey, deployment, apiVersion string, temperature *float32, maxTokens *int) (*AzureOpenAIChatModel, error) {
	endpoint = strings.TrimSpace(endpoint)
	apiKey = strings.TrimSpace(apiKey)
	deployment = strings.TrimSpace(deployment)
	apiVersion = strings.TrimSpace(apiVersion)

	if endpoint == "" {
		return nil, fmt.Errorf("azure openai endpoint is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("azure openai api key is required")
	}
	if deployment == "" {
		return nil, fmt.Errorf("azure openai deployment is required")
	}
	if apiVersion == "" {
		apiVersion = "2025-01-01-preview"
	}

	return &AzureOpenAIChatModel{
		endpoint:    strings.TrimSuffix(endpoint, "/"),
		apiKey:      apiKey,
		apiVersion:  apiVersion,
		deployment:  deployment,
		temperature: temperature,
		maxTokens:   maxTokens,
		tools:       nil,
		client:      azureDefaultClient,
	}, nil
}

func (m *AzureOpenAIChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cp := *m
	// Tools are immutable-ish, but copy slice header to avoid accidental mutation.
	if tools == nil {
		cp.tools = nil
	} else {
		cp.tools = append([]*schema.ToolInfo(nil), tools...)
	}
	return &cp, nil
}

func (m *AzureOpenAIChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	reqMessages, err := schemaMessagesToAzureMessages(input)
	if err != nil {
		return nil, err
	}

	var reqTools []azureTool
	if len(m.tools) > 0 {
		reqTools, err = toolInfosToAzureTools(m.tools)
		if err != nil {
			return nil, err
		}
	}

	body := azureChatCompletionsRequest{
		Messages: reqMessages,
	}
	if m.temperature != nil {
		body.Temperature = m.temperature
	}
	if m.maxTokens != nil {
		body.MaxCompletionTokens = m.maxTokens
	}
	if len(reqTools) > 0 {
		body.Tools = reqTools
		body.ToolChoice = "auto"
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal azure chat request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", m.endpoint, m.deployment, m.apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to create azure chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure chat request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("azure chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var out azureChatCompletionsResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("failed to decode azure chat response: %w", err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("azure chat returned no choices")
	}

	ch := out.Choices[0]
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: ch.Message.Content,
	}

	if len(ch.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, 0, len(ch.Message.ToolCalls))
		for i := range ch.Message.ToolCalls {
			tc := ch.Message.ToolCalls[i]
			idx := i
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{
				Index: &idx,
				ID:    tc.ID,
				Type:  tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: ch.FinishReason,
	}

	// Usage mapping is optional; keep it minimal.
	if out.Usage.TotalTokens != 0 {
		msg.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     out.Usage.PromptTokens,
			CompletionTokens: out.Usage.CompletionTokens,
			TotalTokens:      out.Usage.TotalTokens,
		}
	}

	return msg, nil
}

func (m *AzureOpenAIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reqMessages, err := schemaMessagesToAzureMessages(input)
	if err != nil {
		return nil, err
	}

	var reqTools []azureTool
	if len(m.tools) > 0 {
		reqTools, err = toolInfosToAzureTools(m.tools)
		if err != nil {
			return nil, err
		}
	}

	body := azureChatCompletionsRequest{
		Messages: reqMessages,
		Stream:   true,
	}
	if m.temperature != nil {
		body.Temperature = m.temperature
	}
	if m.maxTokens != nil {
		body.MaxCompletionTokens = m.maxTokens
	}
	if len(reqTools) > 0 {
		body.Tools = reqTools
		body.ToolChoice = "auto"
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal azure stream request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", m.endpoint, m.deployment, m.apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to create azure stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", m.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for streaming
	streamClient := &http.Client{Timeout: 0}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure stream request failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure stream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Create stream reader/writer pair
	sr, sw := schema.Pipe[*schema.Message](100)

	// Start goroutine to read SSE stream
	go func() {
		defer sw.Close()
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)

		// Track accumulated tool calls (they come incrementally)
		toolCallsMap := make(map[int]*azureStreamToolCall)
		var finishReason string

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					sw.Send(nil, err)
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if line == "data: [DONE]" {
				// Stream complete - emit final message with tool calls if any
				if len(toolCallsMap) > 0 {
					finalMsg := &schema.Message{
						Role:      schema.Assistant,
						ToolCalls: make([]schema.ToolCall, 0, len(toolCallsMap)),
					}
					for idx, tc := range toolCallsMap {
						i := idx // capture for pointer
						finalMsg.ToolCalls = append(finalMsg.ToolCalls, schema.ToolCall{
							Index: &i,
							ID:    tc.ID,
							Type:  tc.Type,
							Function: schema.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments.String(),
							},
						})
					}
					finalMsg.ResponseMeta = &schema.ResponseMeta{
						FinishReason: finishReason,
					}
					sw.Send(finalMsg, nil)
				}
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimPrefix(line, "data: ")

			var chunk azureChatStreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			c := chunk.Choices[0]

			// Handle content delta
			if c.Delta.Content != "" {
				sw.Send(&schema.Message{
					Role:    schema.Assistant,
					Content: c.Delta.Content,
				}, nil)
			}

			// Handle tool call deltas (accumulate)
			for _, tc := range c.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}

				if _, exists := toolCallsMap[idx]; !exists {
					toolCallsMap[idx] = &azureStreamToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: azureStreamToolCallFunc{
							Name:      tc.Function.Name,
							Arguments: strings.Builder{},
						},
					}
				}

				existing := toolCallsMap[idx]
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Type != "" {
					existing.Type = tc.Type
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					existing.Function.Arguments.WriteString(tc.Function.Arguments)
				}
			}

			// Track finish reason
			if c.FinishReason != nil && *c.FinishReason != "" {
				finishReason = *c.FinishReason
			}
		}
	}()

	return sr, nil
}

// --- Azure wire types + helpers ---

type azureChatCompletionsRequest struct {
	Messages            []azureMessage `json:"messages"`
	Temperature         *float32       `json:"temperature,omitempty"`
	TopP                *float32       `json:"top_p,omitempty"`
	PresencePenalty     *float32       `json:"presence_penalty,omitempty"`
	FrequencyPenalty    *float32       `json:"frequency_penalty,omitempty"`
	MaxTokens           *int           `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int           `json:"max_completion_tokens,omitempty"`
	Stream              bool           `json:"stream,omitempty"`

	Tools      []azureTool `json:"tools,omitempty"`
	ToolChoice string      `json:"tool_choice,omitempty"`
}

type azureMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`

	// Tool messages
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Assistant messages that contain tool calls
	ToolCalls []azureToolCall `json:"tool_calls,omitempty"`
}

type azureTool struct {
	Type     string              `json:"type"`
	Function azureToolDefinition `json:"function"`
}

type azureToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type azureToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function azureToolCallFunc `json:"function"`
}

type azureToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type azureChatCompletionsResponse struct {
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls []azureToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// --- Azure streaming wire types ---

type azureChatStreamChunk struct {
	Choices []struct {
		Index        int                  `json:"index"`
		Delta        azureChatStreamDelta `json:"delta"`
		FinishReason *string              `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

type azureChatStreamDelta struct {
	Role      string                    `json:"role,omitempty"`
	Content   string                    `json:"content,omitempty"`
	ToolCalls []azureChatStreamToolCall `json:"tool_calls,omitempty"`
}

type azureChatStreamToolCall struct {
	Index    *int                        `json:"index,omitempty"`
	ID       string                      `json:"id,omitempty"`
	Type     string                      `json:"type,omitempty"`
	Function azureChatStreamToolCallFunc `json:"function,omitempty"`
}

type azureChatStreamToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// azureStreamToolCall is used internally to accumulate streamed tool calls
type azureStreamToolCall struct {
	ID       string
	Type     string
	Function azureStreamToolCallFunc
}

type azureStreamToolCallFunc struct {
	Name      string
	Arguments strings.Builder
}

func schemaMessagesToAzureMessages(in []*schema.Message) ([]azureMessage, error) {
	out := make([]azureMessage, 0, len(in))
	for _, m := range in {
		if m == nil {
			continue
		}
		switch m.Role {
		case schema.System, schema.User:
			out = append(out, azureMessage{
				Role:    string(m.Role),
				Content: m.Content,
			})
		case schema.Assistant:
			am := azureMessage{
				Role:    string(m.Role),
				Content: m.Content,
			}
			if len(m.ToolCalls) > 0 {
				am.ToolCalls = make([]azureToolCall, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					am.ToolCalls = append(am.ToolCalls, azureToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: azureToolCallFunc{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
			}
			out = append(out, am)
		case schema.Tool:
			if m.ToolCallID == "" {
				return nil, fmt.Errorf("tool message missing tool_call_id")
			}
			out = append(out, azureMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		default:
			return nil, fmt.Errorf("unsupported role: %s", m.Role)
		}
	}
	return out, nil
}

func toolInfosToAzureTools(in []*schema.ToolInfo) ([]azureTool, error) {
	out := make([]azureTool, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		var params json.RawMessage
		if t.ParamsOneOf != nil {
			js, err := t.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("failed to convert tool params to jsonschema for %q: %w", t.Name, err)
			}
			if js != nil {
				b, err := json.Marshal(js)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool params for %q: %w", t.Name, err)
				}
				params = b
			}
		}

		out = append(out, azureTool{
			Type: "function",
			Function: azureToolDefinition{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		})
	}
	return out, nil
}
