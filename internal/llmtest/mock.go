// Package llmtest provides a scripted eino ToolCallingChatModel stand-in so
// other packages' unit tests can exercise call/parse/control-flow logic
// without hitting a real LLM API.
package llmtest

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// MockResponse is one scripted reply: either Content (a successful
// assistant message) or Err (a failed call).
type MockResponse struct {
	Content string
	Err     error
}

// MockChatModel returns Responses in call order. If Loop is false (the
// default), calling it more times than there are scripted responses is an
// error — tests should script exactly as many responses as they expect
// calls, so an unexpected extra call fails loudly instead of silently
// reusing a stale response.
type MockChatModel struct {
	Responses []MockResponse
	Loop      bool

	calls int32
}

func (m *MockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	idx := int(atomic.AddInt32(&m.calls, 1)) - 1
	if len(m.Responses) == 0 {
		return nil, errors.New("mock model: no scripted responses configured")
	}
	if idx >= len(m.Responses) {
		if !m.Loop {
			return nil, errors.New("mock model: scripted responses exhausted")
		}
		idx = idx % len(m.Responses)
	}
	r := m.Responses[idx]
	if r.Err != nil {
		return nil, r.Err
	}
	return &schema.Message{Role: schema.Assistant, Content: r.Content}, nil
}

func (m *MockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("mock model: streaming not supported")
}

func (m *MockChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// CallCount returns how many times Generate has been called so far.
func (m *MockChatModel) CallCount() int {
	return int(atomic.LoadInt32(&m.calls))
}
