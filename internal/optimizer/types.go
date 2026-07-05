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

	// Patience stops the search early after this many consecutive rejected
	// rounds (a minibatch loss or a reflection error) - a search that isn't
	// finding any improving mutation is unlikely to start after a few more
	// tries, so this saves LLM spend on a stalled run. Patience <= 0 means
	// never stop early - always run the full Iterations budget.
	Patience int

	Concurrency int
	Retries     int
	Seed        int64

	// OnUpdate, if set, is invoked synchronously after every checkpoint
	// (initial seed candidate, each round's history entry) with the
	// in-progress Result. Callers use this to persist partial progress to
	// disk so a crash mid-run doesn't lose everything found so far. Run
	// never mutates Result concurrently with this call.
	OnUpdate func(*Result)
}

// Candidate is one prompt admitted to the pool, with its full score vector
// against every example in the tracking set (the train slice passed to
// Run, in that exact order). Every candidate's Scores must be the same
// length and positionally aligned to that same train slice, since the
// Pareto selection step (pareto.go) compares candidates instance-by-instance.
type Candidate struct {
	ID       int       `json:"id"` // insertion order into the pool; the seed is always 0
	Prompt   string    `json:"prompt"`
	ParentID int       `json:"parent_id"` // -1 for the seed, which has no parent
	Round    int       `json:"round"`     // search round this candidate was accepted on; 0 for the seed
	Scores   []float64 `json:"scores"`    // per-instance score, aligned to train order
	Mean     float64   `json:"mean"`      // mean(Scores), cached to avoid recomputing on every render
}

// IterationRecord captures one round of the search loop: which pool
// candidate was selected as the mutation source, what candidate was tried,
// how it scored against its parent, whether it was accepted, and the
// reflection model's diagnosis. Kept even for rejected rounds so the final
// report shows a full failure-pattern timeline, not just the winners.
type IterationRecord struct {
	Round           int             `json:"round"`
	ParentID        int             `json:"parent_id"` // which pool candidate was selected and mutated this round
	CandidatePrompt string          `json:"candidate_prompt"`
	PriorScore      float64         `json:"prior_score"`     // parent's score on this round's minibatch
	CandidateScore  float64         `json:"candidate_score"` // mutation's score on that same minibatch
	Accepted        bool            `json:"accepted"`
	AcceptedID      int             `json:"accepted_id"` // new pool candidate's ID if Accepted; -1 if rejected
	Analysis        string          `json:"analysis"`
	WorstExamples   []JudgedExample `json:"worst_examples"`
}

// Result is what the search loop produces: the whole candidate pool it
// built, the full round-by-round history, and the pool's best candidate
// (by mean tracking-set score) singled out for convenience since that's
// what the final val-set comparison (internal/evalreport) evaluates.
type Result struct {
	SeedPrompt     string            `json:"seed_prompt"`
	BestPrompt     string            `json:"best_prompt"`
	BestTrainScore float64           `json:"best_train_score"`
	Pool           []Candidate       `json:"pool"`
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
