package judge

import (
	"fmt"
	"strings"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/rubric"
)

// BuildSystemPrompt renders the rubric's dimensions into judge scoring
// instructions and a matching JSON output schema.
func BuildSystemPrompt(r *rubric.Rubric) string {
	var b strings.Builder
	b.WriteString("You are an impartial evaluator judging an AI assistant's response to a user request.\n\n")
	b.WriteString("Score the response on each dimension below, using the full integer range from 0 to the stated scale.\n\n")

	for _, d := range r.Dimensions {
		req := ""
		if d.Required {
			req = " [REQUIRED: a score of 0 here fails the response regardless of other scores]"
		}
		b.WriteString(fmt.Sprintf("- %s (0 to %d): %s%s\n", d.Name, d.Scale, d.Description, req))
	}

	b.WriteString("\nAlso provide:\n")
	b.WriteString("- feedback: a specific, actionable critique of what the response got wrong and why, or what it did well if the score is high. Describe the general failure pattern, not just this one example — it will be used to rewrite the assistant's instructions.\n")
	b.WriteString("- hallucination_flag: true if the response asserts something not supported by the input or the reference material.\n\n")
	b.WriteString("If a reference/ground-truth is provided, treat it as the source of truth for factual and policy claims.\n\n")

	b.WriteString("Return ONLY valid JSON in this exact shape:\n{\n  \"scores\": {\n")
	for i, d := range r.Dimensions {
		comma := ","
		if i == len(r.Dimensions)-1 {
			comma = ""
		}
		b.WriteString(fmt.Sprintf("    %q: 0%s\n", d.Name, comma))
	}
	b.WriteString("  },\n  \"feedback\": \"\",\n  \"hallucination_flag\": false\n}")

	return b.String()
}

// BuildUserPrompt renders the example (optional history/reference) and the
// candidate's response for the judge to evaluate.
func BuildUserPrompt(ex dataset.Example, candidateOutput string) string {
	var b strings.Builder

	if len(ex.History) > 0 {
		b.WriteString("Conversation so far:\n")
		for _, turn := range ex.History {
			b.WriteString(fmt.Sprintf("%s: %s\n", turn.Role, turn.Content))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("User input:\n%s\n\n", ex.Input))

	if ex.Reference != "" {
		b.WriteString(fmt.Sprintf("Reference / ground truth:\n%s\n\n", ex.Reference))
	}

	b.WriteString(fmt.Sprintf("Assistant response to judge:\n%s\n", candidateOutput))
	return b.String()
}
