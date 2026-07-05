package judge

import (
	"context"
	"errors"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/llmtest"
	"github.com/Conversly/prompt-opt/internal/rubric"
)

func testRubric() *rubric.Rubric {
	return &rubric.Rubric{
		Dimensions: []rubric.Dimension{
			{Name: "accuracy", Description: "correct", Scale: 1, Weight: 3, Required: true},
			{Name: "tone", Description: "warm", Scale: 5, Weight: 1, Required: false},
		},
		PassThreshold: 0.5,
	}
}

func testExample() dataset.Example {
	return dataset.Example{ID: "case-1", Input: "hello"}
}

func TestScoreParsesCleanJSON(t *testing.T) {
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"scores": {"accuracy": 1, "tone": 4}, "feedback": "good", "hallucination_flag": false}`},
	}}
	j := New(mock, testRubric(), 1)

	v, err := j.Score(context.Background(), testExample(), "candidate output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// weighted: accuracy 1/1*3=3, tone 4/5*1=0.8 => 3.8/4 = 0.95
	if v.Overall < 0.94 || v.Overall > 0.96 {
		t.Fatalf("expected overall ~0.95, got %v", v.Overall)
	}
	if !v.Pass {
		t.Fatalf("expected pass=true, got false")
	}
}

func TestScoreParsesMarkdownFencedJSON(t *testing.T) {
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: "```json\n{\"scores\": {\"accuracy\": 1, \"tone\": 5}, \"feedback\": \"ok\"}\n```"},
	}}
	j := New(mock, testRubric(), 0)

	v, err := j.Score(context.Background(), testExample(), "candidate output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Feedback != "ok" {
		t.Fatalf("expected feedback 'ok', got %q", v.Feedback)
	}
}

func TestScoreRetriesOnInvalidJSONThenSucceeds(t *testing.T) {
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: "not json at all"},
		{Content: `{"scores": {"accuracy": 1, "tone": 3}}`},
	}}
	j := New(mock, testRubric(), 1)

	v, err := j.Score(context.Background(), testExample(), "candidate output")
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if v == nil {
		t.Fatalf("expected a verdict")
	}
	if mock.CallCount() != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", mock.CallCount())
	}
}

func TestScoreFailsAfterExhaustingRetries(t *testing.T) {
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: "not json"},
		{Content: "still not json"},
	}}
	j := New(mock, testRubric(), 1)

	_, err := j.Score(context.Background(), testExample(), "candidate output")
	if err == nil {
		t.Fatalf("expected error after exhausting retries")
	}
}

func TestScorePropagatesTransportError(t *testing.T) {
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Err: errors.New("network exploded")},
	}}
	j := New(mock, testRubric(), 0)

	_, err := j.Score(context.Background(), testExample(), "candidate output")
	if err == nil {
		t.Fatalf("expected error to propagate")
	}
}

func TestScoreRequiredDimensionZeroFailsRegardlessOfOverall(t *testing.T) {
	r := &rubric.Rubric{
		Dimensions: []rubric.Dimension{
			{Name: "required_dim", Scale: 1, Weight: 1, Required: true},
			{Name: "bonus_dim", Scale: 1, Weight: 100, Required: false},
		},
		PassThreshold: 0, // trivially satisfied by the weighted average alone
	}
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"scores": {"required_dim": 0, "bonus_dim": 1}}`},
	}}
	j := New(mock, r, 0)

	v, err := j.Score(context.Background(), testExample(), "candidate output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Pass {
		t.Fatalf("expected pass=false because a required dimension scored 0, got true (overall=%v)", v.Overall)
	}
}

func TestScoreClampsOutOfRangeScores(t *testing.T) {
	r := &rubric.Rubric{
		Dimensions:    []rubric.Dimension{{Name: "accuracy", Scale: 5, Weight: 1, Required: false}},
		PassThreshold: 0,
	}
	mock := &llmtest.MockChatModel{Responses: []llmtest.MockResponse{
		{Content: `{"scores": {"accuracy": 999}}`},
	}}
	j := New(mock, r, 0)

	v, err := j.Score(context.Background(), testExample(), "candidate output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Scores["accuracy"] != 5 {
		t.Fatalf("expected clamped score of 5, got %v", v.Scores["accuracy"])
	}
	if v.Overall != 1 {
		t.Fatalf("expected overall 1.0 after clamping, got %v", v.Overall)
	}
}
