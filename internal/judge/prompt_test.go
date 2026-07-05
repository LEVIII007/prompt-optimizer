package judge

import (
	"strings"
	"testing"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/rubric"
)

func TestBuildSystemPromptIncludesDimensions(t *testing.T) {
	r := &rubric.Rubric{
		Dimensions: []rubric.Dimension{
			{Name: "policy_accuracy", Description: "states correct policy", Scale: 1, Weight: 3, Required: true},
			{Name: "tone", Description: "professional and warm", Scale: 5, Weight: 1, Required: false},
		},
		PassThreshold: 0.75,
	}

	prompt := BuildSystemPrompt(r)

	for _, want := range []string{"policy_accuracy", "states correct policy", "0 to 1", "tone", "professional and warm", "0 to 5", "[REQUIRED"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected system prompt to contain %q, got:\n%s", want, prompt)
		}
	}

	// tone is not required, so it must not be tagged as such.
	if strings.Contains(prompt, `tone (0 to 5): professional and warm [REQUIRED`) {
		t.Errorf("did not expect tone to be marked required, got:\n%s", prompt)
	}
}

func TestBuildUserPromptIncludesHistoryAndReference(t *testing.T) {
	ex := dataset.Example{
		ID:    "case-1",
		Input: "Can I get a refund after 40 days?",
		History: []dataset.Turn{
			{Role: "user", Content: "earlier question"},
			{Role: "assistant", Content: "earlier answer"},
		},
		Reference: "Refunds are allowed within 30 days.",
	}
	got := BuildUserPrompt(ex, "candidate response text")

	for _, want := range []string{"Conversation so far", "user: earlier question", "assistant: earlier answer", ex.Input, ex.Reference, "candidate response text"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected user prompt to contain %q, got:\n%s", want, got)
		}
	}
}
