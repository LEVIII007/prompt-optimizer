package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/llm"
	"github.com/Conversly/prompt-opt/internal/rubric"
	"github.com/Conversly/prompt-opt/internal/utils"
)

const callTimeout = 90 * time.Second

// Judge scores candidate responses against a rubric using an LLM.
type Judge struct {
	model   model.ToolCallingChatModel
	rubric  *rubric.Rubric
	retries int
}

func New(m model.ToolCallingChatModel, r *rubric.Rubric, retries int) *Judge {
	return &Judge{model: m, rubric: r, retries: retries}
}

// Score asks the judge model to evaluate candidateOutput against ex. The
// whole call+parse attempt is retried up to `retries` additional times on
// either a transport error or invalid JSON, since a malformed response is
// often just as transient as a dropped connection.
func (j *Judge) Score(ctx context.Context, ex dataset.Example, candidateOutput string) (*Verdict, error) {
	messages := []*schema.Message{
		schema.SystemMessage(BuildSystemPrompt(j.rubric)),
		schema.UserMessage(BuildUserPrompt(ex, candidateOutput)),
	}

	var lastErr error
	attempts := j.retries + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		v, err := j.attempt(ctx, messages)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("judge failed after %d attempt(s): %w", attempts, lastErr)
}

func (j *Judge) attempt(ctx context.Context, messages []*schema.Message) (*Verdict, error) {
	res, err := llm.GenerateWithRetry(ctx, j.model, messages, 0, callTimeout)
	if err != nil {
		return nil, fmt.Errorf("judge call failed: %w", err)
	}

	raw := utils.CleanModelJSON(res.Content)
	if raw == "" {
		return nil, fmt.Errorf("judge returned empty content")
	}

	var v Verdict
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, fmt.Errorf("judge returned invalid JSON: %w", err)
	}

	j.finalize(&v)
	return &v, nil
}

// finalize clamps per-dimension scores into [0, scale], then computes the
// weighted-average Overall (0..1) and Pass. The LLM's own opinion of
// "overall"/"pass", if it offered one, is discarded in favor of this.
func (j *Judge) finalize(v *Verdict) {
	if v.Scores == nil {
		v.Scores = map[string]float64{}
	}

	var weightedSum float64
	pass := true
	for _, d := range j.rubric.Dimensions {
		score := v.Scores[d.Name]
		if score < 0 {
			score = 0
		}
		if score > float64(d.Scale) {
			score = float64(d.Scale)
		}
		v.Scores[d.Name] = score

		normalized := score / float64(d.Scale)
		weightedSum += normalized * d.Weight

		if d.Required && normalized <= 0 {
			pass = false
		}
	}

	if totalWeight := j.rubric.TotalWeight(); totalWeight > 0 {
		v.Overall = weightedSum / totalWeight
	}
	if v.Overall < j.rubric.PassThreshold {
		pass = false
	}
	v.Pass = pass
}
