package evalreport

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/llmtest"
	"github.com/Conversly/prompt-opt/internal/optimizer"
	"github.com/Conversly/prompt-opt/internal/rubric"
)

func flatRubric() *rubric.Rubric {
	return &rubric.Rubric{
		Dimensions:    []rubric.Dimension{{Name: "quality", Scale: 10, Weight: 1, Required: false}},
		PassThreshold: 0,
	}
}

func scoreResponse(score int) llmtest.MockResponse {
	return llmtest.MockResponse{Content: fmt.Sprintf(`{"scores": {"quality": %d}, "feedback": "fb"}`, score)}
}

func TestEvaluateComputesAggregateAndDelta(t *testing.T) {
	val := []dataset.Example{
		{ID: "v1", Input: "in1"},
		{ID: "v2", Input: "in2"},
	}
	// seed phase: 2 calls @ score 3 (Overall 0.3); best phase: 2 calls @ score 9 (Overall 0.9).
	judgeMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		scoreResponse(3), scoreResponse(3), scoreResponse(9), scoreResponse(9),
	}}
	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "resp"}}, Loop: true}
	j := judge.New(judgeMock, flatRubric(), 0)
	settings := optimizer.Settings{Concurrency: 1, Retries: 0}

	cmp, err := Evaluate(context.Background(), taskMock, j, "seed", "best", val, 0.9, settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff := cmp.SeedAggregate - 0.3; diff > 0.01 || diff < -0.01 {
		t.Fatalf("expected seed aggregate ~0.3, got %v", cmp.SeedAggregate)
	}
	if diff := cmp.BestAggregate - 0.9; diff > 0.01 || diff < -0.01 {
		t.Fatalf("expected best aggregate ~0.9, got %v", cmp.BestAggregate)
	}
	if diff := cmp.Delta - 0.6; diff > 0.02 || diff < -0.02 {
		t.Fatalf("expected delta ~0.6, got %v", cmp.Delta)
	}
	if cmp.TrainValGapWarning {
		t.Fatalf("expected no gap warning when train score matches val score")
	}
	if len(cmp.PerExample) != 2 {
		t.Fatalf("expected 2 per-example scores, got %d", len(cmp.PerExample))
	}
}

func TestEvaluateFlagsTrainValGap(t *testing.T) {
	val := []dataset.Example{{ID: "v1", Input: "in1"}}
	judgeMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{scoreResponse(3), scoreResponse(5)}}
	taskMock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{{Content: "resp"}}, Loop: true}
	j := judge.New(judgeMock, flatRubric(), 0)
	settings := optimizer.Settings{Concurrency: 1, Retries: 0}

	// best phase scores 0.5 on val, but the optimizer claimed 0.95 on train.
	cmp, err := Evaluate(context.Background(), taskMock, j, "seed", "best", val, 0.95, settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmp.TrainValGapWarning {
		t.Fatalf("expected gap warning: train=0.95 vs val=%.2f", cmp.BestAggregate)
	}
}

func TestEvaluateRejectsEmptyValSet(t *testing.T) {
	j := judge.New(&llmtest.MockChatModel{}, flatRubric(), 0)
	_, err := Evaluate(context.Background(), &llmtest.MockChatModel{}, j, "seed", "best", nil, 0, optimizer.Settings{Concurrency: 1})
	if err == nil {
		t.Fatalf("expected an error for an empty validation set")
	}
}

func TestAggregateByCategoryAveragesPerCategory(t *testing.T) {
	judged := []optimizer.JudgedExample{
		{Example: dataset.Example{ID: "1", Category: "a"}, Verdict: &judge.Verdict{Overall: 0.2}},
		{Example: dataset.Example{ID: "2", Category: "a"}, Verdict: &judge.Verdict{Overall: 0.4}},
		{Example: dataset.Example{ID: "3", Category: "b"}, Verdict: &judge.Verdict{Overall: 1.0}},
	}

	got := aggregateByCategory(judged)
	if diff := got["a"] - 0.3; diff > 0.001 || diff < -0.001 {
		t.Fatalf("expected category a average 0.3, got %v", got["a"])
	}
	if got["b"] != 1.0 {
		t.Fatalf("expected category b average 1.0, got %v", got["b"])
	}
}

func TestMergePerExampleZipsByIndex(t *testing.T) {
	seed := []optimizer.JudgedExample{
		{Example: dataset.Example{ID: "1", Category: "a"}, Verdict: &judge.Verdict{Overall: 0.2, Pass: false}},
	}
	best := []optimizer.JudgedExample{
		{Example: dataset.Example{ID: "1", Category: "a"}, Verdict: &judge.Verdict{Overall: 0.8, Pass: true}},
	}

	got := mergePerExample(seed, best)
	if len(got) != 1 {
		t.Fatalf("expected 1 merged example, got %d", len(got))
	}
	if got[0].SeedScore != 0.2 || got[0].BestScore != 0.8 || got[0].SeedPass || !got[0].BestPass {
		t.Fatalf("unexpected merged example: %+v", got[0])
	}
}

func TestRenderMarkdownIncludesKeySections(t *testing.T) {
	result := &optimizer.Result{
		SeedPrompt:     "seed",
		BestPrompt:     "best",
		BestTrainScore: 0.9,
		Pool: []optimizer.Candidate{
			{ID: 0, Prompt: "seed", ParentID: -1, Round: 0, Mean: 0.3},
			{ID: 1, Prompt: "best", ParentID: 0, Round: 1, Mean: 0.9},
		},
		History: []optimizer.IterationRecord{
			{Round: 1, ParentID: 0, Accepted: true, AcceptedID: 1, PriorScore: 0.3, CandidateScore: 0.7, Analysis: "fixed tone", WorstExamples: []optimizer.JudgedExample{
				{Example: dataset.Example{ID: "v1", Category: "refund"}, Output: "sorry, no refunds ever", Verdict: &judge.Verdict{Overall: 0.3, Feedback: "too rigid, ignores exceptions"}},
			}},
		},
	}
	cmp := &Comparison{
		SeedAggregate:      0.3,
		BestAggregate:      0.7,
		Delta:              0.4,
		TrainValGapWarning: true,
		BestTrainScore:     0.9,
		BestValScore:       0.7,
		SeedByCategory:     map[string]float64{"refund": 0.3},
		BestByCategory:     map[string]float64{"refund": 0.7},
		PerExample: []ExampleScore{
			{ID: "v1", Category: "refund", SeedScore: 0.3, BestScore: 0.7},
		},
	}

	md := RenderMarkdown(result, cmp)

	for _, want := range []string{
		"Prompt Optimization Report", "Warning", "refund", "Round 1", "fixed tone", "v1", "ignores exceptions", "no refunds ever",
		"Candidate pool", "#1", "winner", "Admitted to pool as candidate `#1`",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("expected markdown report to contain %q, got:\n%s", want, md)
		}
	}
}
