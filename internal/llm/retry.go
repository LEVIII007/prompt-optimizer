package llm

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// GenerateWithRetry calls model.Generate, retrying up to `retries` additional
// times (so retries=0 means a single attempt) on error or an empty response.
// Each attempt gets its own timeout so one hung call can't eat the whole budget.
func GenerateWithRetry(ctx context.Context, m model.ToolCallingChatModel, messages []*schema.Message, retries int, timeout time.Duration) (*schema.Message, error) {
	var lastErr error
	attempts := retries + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		res, err := m.Generate(callCtx, messages)
		cancel()
		if err == nil && res != nil {
			return res, nil
		}
		if err == nil {
			err = errors.New("empty model response")
		}
		lastErr = err
	}
	return nil, lastErr
}
