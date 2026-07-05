package optimizer

import (
	"github.com/cloudwego/eino/components/model"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/judge"
)

// Deps bundles the model roles the search loop needs beyond the judge
// (which is passed separately since it already bundles a model + rubric).
// TaskModel runs candidate prompts; ReflectionModel proposes rewrites from
// failures. They may be the same underlying deployment.
type Deps struct {
	TaskModel       model.ToolCallingChatModel
	ReflectionModel model.ToolCallingChatModel
}

// Settings controls the search loop's behavior.
type Settings struct {
	Iterations    int
	MinibatchSize int
	Patience      int
	FullEvalEvery int
	Concurrency   int
	Retries       int
	Seed          int64
}

// IterationRecord captures one round of the search loop: what candidate was
// tried, how it scored against the prior best, whether it was accepted, and
// the reflection model's diagnosis. Kept even for rejected rounds so the
// final report shows a full failure-pattern timeline, not just the winners.
type IterationRecord struct {
	Round           int             `json:"round"`
	CandidatePrompt string          `json:"candidate_prompt"`
	PriorScore      float64         `json:"prior_score"`
	CandidateScore  float64         `json:"candidate_score"`
	Accepted        bool            `json:"accepted"`
	Analysis        string          `json:"analysis"`
	WorstExamples   []JudgedExample `json:"worst_examples"`
}

// Result is what the search loop produces: the best candidate found (which
// may just be the seed, if nothing ever beat it) and the full history.
type Result struct {
	SeedPrompt     string            `json:"seed_prompt"`
	BestPrompt     string            `json:"best_prompt"`
	BestTrainScore float64           `json:"best_train_score"`
	History        []IterationRecord `json:"history"`
}

// JudgedExample bundles one example's execution output and judge verdict —
// the unit the reflection step, worst-K selection, and the final val-set
// comparison (internal/evalreport) all operate on. It's also persisted
// verbatim in IterationRecord.WorstExamples, so run_history.json carries the
// judge's per-dimension scores and feedback for the examples reflection saw.
type JudgedExample struct {
	Example dataset.Example `json:"example"`
	Output  string          `json:"output"`
	Verdict *judge.Verdict  `json:"verdict"`
}
