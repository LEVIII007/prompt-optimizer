package evalreport

import (
	"strings"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/optimizer"
)

func TestRenderHTMLIncludesKeySections(t *testing.T) {
	result := &optimizer.Result{
		SeedPrompt:     "seed prompt text",
		BestPrompt:     "best prompt text",
		BestTrainScore: 0.9,
		Pool: []optimizer.Candidate{
			{ID: 0, Prompt: "seed prompt text", ParentID: -1, Round: 0, Mean: 0.3},
			{ID: 1, Prompt: "best prompt text", ParentID: 0, Round: 1, Mean: 0.9},
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
			{ID: "v1", Category: "refund", SeedScore: 0.3, BestScore: 0.7, SeedPass: false, BestPass: true},
		},
	}

	out := RenderHTML(result, cmp)

	for _, want := range []string{
		"<!doctype html>", "Prompt Optimization Report", "Possible overfitting",
		"refund", "Round 1", "fixed tone", "v1", "ignores exceptions", "no refunds ever",
		"seed prompt text", "best prompt text",
		"Candidate pool", "#1", "winner", "admitted as #1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected HTML report to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRenderHTMLEscapesUntrustedContent(t *testing.T) {
	malicious := `<script>alert(1)</script>`
	result := &optimizer.Result{
		SeedPrompt: malicious,
		BestPrompt: "best",
		History: []optimizer.IterationRecord{
			{Round: 1, Accepted: false, Analysis: malicious, WorstExamples: []optimizer.JudgedExample{
				{Example: dataset.Example{ID: malicious, Category: "cat"}, Output: malicious, Verdict: &judge.Verdict{Overall: 0.1, Feedback: malicious}},
			}},
		},
	}
	cmp := &Comparison{PerExample: []ExampleScore{{ID: "v1", Category: "cat", SeedScore: 0.1, BestScore: 0.1}}}

	out := RenderHTML(result, cmp)

	if strings.Contains(out, malicious) {
		t.Fatalf("expected judge/LLM-controlled content to be HTML-escaped, but raw <script> tag survived in output:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("expected escaped form &lt;script&gt; to be present, got:\n%s", out)
	}
}
