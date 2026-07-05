package executor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/llm"
	"github.com/Conversly/prompt-opt/internal/utils"
)

const callTimeout = 90 * time.Second

// Result is a candidate prompt's response to one dataset example.
type Result struct {
	Example dataset.Example
	Output  string
	Err     error
}

// Run executes systemPrompt against every example concurrently (bounded by
// concurrency) using m, and returns one Result per example in the same
// order as examples — a buffered-channel semaphore plus index-addressed
// writes, same pattern as the response repo's chat-e2e-bench runner.
func Run(ctx context.Context, m model.ToolCallingChatModel, systemPrompt string, examples []dataset.Example, concurrency int, retries int) []Result {
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]Result, len(examples))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var done int64

	for i, ex := range examples {
		wg.Add(1)
		go func(idx int, example dataset.Example) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			output, err := runOne(ctx, m, systemPrompt, example, retries)
			results[idx] = Result{Example: example, Output: output, Err: err}
			utils.Logger().Info("task call complete",
				zap.Int64("done", atomic.AddInt64(&done, 1)), zap.Int("total", len(examples)),
				zap.String("example_id", example.ID), zap.Bool("error", err != nil))
		}(i, ex)
	}

	wg.Wait()
	return results
}

func runOne(ctx context.Context, m model.ToolCallingChatModel, systemPrompt string, ex dataset.Example, retries int) (string, error) {
	res, err := llm.GenerateWithRetry(ctx, m, buildMessages(systemPrompt, ex), retries, callTimeout)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

func buildMessages(systemPrompt string, ex dataset.Example) []*schema.Message {
	messages := make([]*schema.Message, 0, len(ex.History)+2)
	messages = append(messages, schema.SystemMessage(systemPrompt))
	for _, turn := range ex.History {
		role := schema.User
		if turn.Role == "assistant" {
			role = schema.Assistant
		}
		messages = append(messages, &schema.Message{Role: role, Content: turn.Content})
	}
	messages = append(messages, schema.UserMessage(ex.Input))
	return messages
}
