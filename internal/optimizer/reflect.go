package optimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Conversly/prompt-opt/internal/llm"
	"github.com/Conversly/prompt-opt/internal/utils"
)

const reflectTimeout = 120 * time.Second

const reflectionSystemPrompt = `You are improving a system prompt for an AI assistant based on evaluation failures.

You will be shown the current system prompt and a set of examples where it produced a low-scoring response, each with the judge's per-dimension scores and feedback.

Task:
1. Diagnose the recurring pattern(s) behind these failures. Be specific about what instruction is missing, ambiguous, or wrong in the current prompt - not just what went wrong in one example.
2. Rewrite the ENTIRE system prompt to fix these patterns. Generalize the fix; do not patch in a rule that only matches the literal examples shown.
3. Prefer editing or merging existing instructions over appending new ones. If a new rule overlaps with or restates something already in the prompt, fold it into that existing instruction instead of adding a separate section - do not grow the prompt just to say the same thing in different words.
4. Keep the prompt as short as it can be while still fixing the diagnosed failures. A long, checklist-heavy prompt is usually a symptom of unmerged, overlapping rules, not thoroughness - actively look for redundant or overlapping instructions already in the prompt and consolidate them, even if they weren't the cause of this round's failures.
5. Do not let a new rule bury or override a higher-priority default in some situations (for example, a rigid "always state policy/facts first" instruction should not push safety guidance, empathy, or a direct answer to the customer's actual question further down the response). If a new rule could conflict with an existing one, say explicitly which one takes precedence and when.
6. Do not remove behavioral coverage unrelated to the diagnosed failures - only remove or merge instructions that are redundant restatements of something already covered, not instructions that handle a distinct case.

Return ONLY valid JSON in this shape:
{
  "analysis": "diagnosis of the recurring failure pattern(s)",
  "revised_prompt": "the complete new system prompt"
}`

// Reflect asks the reflection model to diagnose why currentPrompt failed on
// worst and propose a complete revised prompt. The whole call+parse attempt
// is retried up to `retries` additional times on transport error or invalid
// JSON, same rationale as judge.Score.
func Reflect(ctx context.Context, m model.ToolCallingChatModel, currentPrompt string, worst []JudgedExample, retries int) (revisedPrompt string, analysis string, err error) {
	messages := []*schema.Message{
		schema.SystemMessage(reflectionSystemPrompt),
		schema.UserMessage(buildReflectionUserPrompt(currentPrompt, worst)),
	}

	var lastErr error
	attempts := retries + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		revised, an, err := reflectAttempt(ctx, m, messages)
		if err == nil {
			return revised, an, nil
		}
		lastErr = err
	}
	return "", "", fmt.Errorf("reflection failed after %d attempt(s): %w", attempts, lastErr)
}

func reflectAttempt(ctx context.Context, m model.ToolCallingChatModel, messages []*schema.Message) (string, string, error) {
	res, err := llm.GenerateWithRetry(ctx, m, messages, 0, reflectTimeout)
	if err != nil {
		return "", "", fmt.Errorf("reflection call failed: %w", err)
	}

	raw := utils.CleanModelJSON(res.Content)
	if raw == "" {
		return "", "", fmt.Errorf("reflection returned empty content")
	}

	var out struct {
		Analysis      string `json:"analysis"`
		RevisedPrompt string `json:"revised_prompt"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", "", fmt.Errorf("reflection returned invalid JSON: %w", err)
	}
	if strings.TrimSpace(out.RevisedPrompt) == "" {
		return "", "", fmt.Errorf("reflection returned an empty revised_prompt")
	}
	return out.RevisedPrompt, out.Analysis, nil
}

func buildReflectionUserPrompt(currentPrompt string, worst []JudgedExample) string {
	var b strings.Builder
	b.WriteString("Current system prompt:\n\"\"\"\n")
	b.WriteString(currentPrompt)
	b.WriteString("\n\"\"\"\n\n")
	b.WriteString("Low-scoring examples:\n\n")
	for i, je := range worst {
		b.WriteString(fmt.Sprintf("Example %d (id=%s):\n", i+1, je.Example.ID))
		b.WriteString(fmt.Sprintf("User input: %s\n", je.Example.Input))
		if je.Example.Reference != "" {
			b.WriteString(fmt.Sprintf("Reference: %s\n", je.Example.Reference))
		}
		b.WriteString(fmt.Sprintf("Assistant response: %s\n", je.Output))
		if je.Verdict != nil {
			b.WriteString(fmt.Sprintf("Scores: %v (overall=%.2f)\n", je.Verdict.Scores, je.Verdict.Overall))
			b.WriteString(fmt.Sprintf("Judge feedback: %s\n", je.Verdict.Feedback))
		}
		b.WriteString("\n")
	}
	return b.String()
}
