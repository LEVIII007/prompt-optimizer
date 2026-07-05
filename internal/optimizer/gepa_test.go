package optimizer

import (
	"context"
	"fmt"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/llmtest"
	"github.com/Conversly/prompt-opt/internal/rubric"
)

func testTrainSet(n int) []dataset.Example {
	examples := make([]dataset.Example, n)
	for i := range examples {
		examples[i] = dataset.Example{ID: fmt.Sprintf("ex-%d", i), Input: fmt.Sprintf("input-%d", i)}
	}
	return examples
}

func flatRubric() *rubric.Rubric {
	return &rubric.Rubric{
		Dimensions:    []rubric.Dimension{{Name: "quality", Scale: 10, Weight: 1, Required: false}},
		PassThreshold: 0,
	}
}

func scoreResponse(score int) llmtest.MockResponse {
	return llmtest.MockResponse{Content: fmt.Sprintf(`{"scores": {"quality": %d}, "feedback": "fb"}`, score)}
}

func repeat(r llmtest.MockResponse, n int) []llmtest.MockResponse {
	out := make([]llmtest.MockResponse, n)
	for i := range out {
		out[i] = r
	}
	return out
}

// With a 4-example train set and minibatch size 4 (== full train), every
// evaluateOnSet call inside Run touches exactly 4 examples. Since all 4
// judge calls within a single phase are scripted to return the identical
// score, the result is independent of goroutine scheduling order - only the
// order of *phases* (baseline, prior, candidate, full-recheck) matters, and
// those run sequentially.
func TestRunAcceptsImprovingCandidate(t *testing.T) {
	train := testTrainSet(4)

	var judgeResponses []llmtest.MockResponse
	judgeResponses = append(judgeResponses, repeat(scoreResponse(3), 8)...) // baseline + prior
	judgeResponses = append(judgeResponses, repeat(scoreResponse(9), 8)...) // candidate + full-recheck
	judgeMock := &llmtest.MockChatModel{Responses: judgeResponses}

	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "response"}}, Loop: true}
	reflectionMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"analysis": "fixed missing instruction", "revised_prompt": "candidate v1 text"}`},
	}}

	j := judge.New(judgeMock, flatRubric(), 0)
	deps := Deps{TaskModel: taskMock, ReflectionModel: reflectionMock}
	settings := Settings{Iterations: 1, MinibatchSize: 4, Patience: 10, FullEvalEvery: 1, Concurrency: 2, Retries: 0, Seed: 1}

	result, err := Run(context.Background(), deps, j, "seed prompt", train, settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BestPrompt != "candidate v1 text" {
		t.Fatalf("expected accepted candidate to become best, got %q", result.BestPrompt)
	}
	if len(result.History) != 1 || !result.History[0].Accepted {
		t.Fatalf("expected 1 accepted history record, got %+v", result.History)
	}
	if result.BestTrainScore < 0.85 {
		t.Fatalf("expected best train score ~0.9, got %v", result.BestTrainScore)
	}
	if len(result.History[0].WorstExamples) != worstK {
		t.Fatalf("expected %d worst examples recorded, got %d", worstK, len(result.History[0].WorstExamples))
	}
	for _, je := range result.History[0].WorstExamples {
		if je.Verdict == nil || je.Verdict.Feedback != "fb" {
			t.Fatalf("expected worst example to carry the judge's verdict/feedback, got %+v", je)
		}
	}
}

func TestRunRejectsNonImprovingCandidate(t *testing.T) {
	train := testTrainSet(4)

	judgeMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{scoreResponse(5)}, Loop: true}
	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "response"}}, Loop: true}
	reflectionMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"analysis": "tried something", "revised_prompt": "candidate v1 text"}`},
	}, Loop: true}

	j := judge.New(judgeMock, flatRubric(), 0)
	deps := Deps{TaskModel: taskMock, ReflectionModel: reflectionMock}
	settings := Settings{Iterations: 1, MinibatchSize: 4, Patience: 10, FullEvalEvery: 1, Concurrency: 2, Retries: 0, Seed: 1}

	result, err := Run(context.Background(), deps, j, "seed prompt", train, settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BestPrompt != "seed prompt" {
		t.Fatalf("expected seed prompt to remain best when candidate doesn't improve, got %q", result.BestPrompt)
	}
	if len(result.History) != 1 || result.History[0].Accepted {
		t.Fatalf("expected 1 rejected history record, got %+v", result.History)
	}
}

func TestRunStopsEarlyOnPatience(t *testing.T) {
	train := testTrainSet(4)

	judgeMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{scoreResponse(5)}, Loop: true}
	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "response"}}, Loop: true}
	reflectionMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"analysis": "tried something", "revised_prompt": "candidate text"}`},
	}, Loop: true}

	j := judge.New(judgeMock, flatRubric(), 0)
	deps := Deps{TaskModel: taskMock, ReflectionModel: reflectionMock}
	settings := Settings{Iterations: 5, MinibatchSize: 4, Patience: 2, FullEvalEvery: 1, Concurrency: 2, Retries: 0, Seed: 1}

	result, err := Run(context.Background(), deps, j, "seed prompt", train, settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.History) != 2 {
		t.Fatalf("expected early stop after 2 rounds (patience=2), got %d rounds", len(result.History))
	}
}

func TestRunSurvivesReflectionError(t *testing.T) {
	train := testTrainSet(4)

	judgeMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{scoreResponse(5)}, Loop: true}
	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "response"}}, Loop: true}
	reflectionMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "not json"}}, Loop: true}

	j := judge.New(judgeMock, flatRubric(), 0)
	deps := Deps{TaskModel: taskMock, ReflectionModel: reflectionMock}
	settings := Settings{Iterations: 2, MinibatchSize: 4, Patience: 10, FullEvalEvery: 1, Concurrency: 2, Retries: 0, Seed: 1}

	result, err := Run(context.Background(), deps, j, "seed prompt", train, settings)
	if err != nil {
		t.Fatalf("expected Run to tolerate reflection errors, got: %v", err)
	}
	if result.BestPrompt != "seed prompt" {
		t.Fatalf("expected seed prompt to remain best when reflection always fails, got %q", result.BestPrompt)
	}
	if len(result.History) != 2 {
		t.Fatalf("expected a history record per round even on reflection failure, got %d", len(result.History))
	}
}
