package executor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/llmtest"
)

func TestRunPreservesOrderUnderConcurrency(t *testing.T) {
	n := 20
	examples := make([]dataset.Example, n)
	for i := 0; i < n; i++ {
		examples[i] = dataset.Example{ID: fmt.Sprintf("case-%d", i), Input: fmt.Sprintf("input-%d", i)}
	}
	// Responses are handed out in call order, which under concurrency=5 is
	// not the same as example order — this guards against a bug where a
	// result gets written to the wrong slot instead of its own example's index.
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "output"}}, Loop: true}

	results := Run(context.Background(), mock, "system prompt", examples, 5, 0)

	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	for i, r := range results {
		if r.Example.ID != examples[i].ID {
			t.Fatalf("result %d: expected example %s, got %s", i, examples[i].ID, r.Example.ID)
		}
	}
}

func TestRunIsolatesPerExampleErrors(t *testing.T) {
	examples := []dataset.Example{
		{ID: "ok-1", Input: "a"},
		{ID: "fails", Input: "b"},
		{ID: "ok-2", Input: "c"},
	}
	// Which example lands on the error response is scheduling-dependent
	// (goroutines race for the semaphore regardless of concurrency), so this
	// only asserts the order-independent invariants: every result stays tied
	// to its own example, and exactly one of the three calls fails.
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: "fine"},
		{Err: errors.New("boom")},
		{Content: "fine"},
	}}

	results := Run(context.Background(), mock, "system prompt", examples, 1, 0)

	errCount := 0
	for i, r := range results {
		if r.Example.ID != examples[i].ID {
			t.Fatalf("result %d: expected example %s, got %s", i, examples[i].ID, r.Example.ID)
		}
		if r.Err != nil {
			errCount++
		}
	}
	if errCount != 1 {
		t.Fatalf("expected exactly 1 failing example, got %d failures across %+v", errCount, results)
	}
}
