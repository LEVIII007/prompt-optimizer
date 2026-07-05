package optimizer

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/executor"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/utils"
)

// worstK is how many of the lowest-scoring minibatch examples get shown to
// the reflection model each round.
const worstK = 3

// Run performs GEPA's reflective-mutation search: starting from a pool
// containing just seedPrompt, each round selects a parent from the pool via
// Pareto-frontier-weighted sampling (pareto.go), mutates it via reflection,
// and accepts the mutation only if it beats its parent on a freshly-sampled
// minibatch. An accepted mutation is evaluated against the whole train set
// once - that full per-instance score vector is what both the next round's
// selection and the final winner choice depend on - and joins the pool. It
// is never discarded and never overwrites another candidate, so a mutation
// that trades broad coverage for a few examples can't silently erase what an
// earlier candidate was good at. Run never touches a validation set - that
// comparison happens separately, once, after Run returns.
func Run(ctx context.Context, deps Deps, j *judge.Judge, seedPrompt string, train []dataset.Example, settings Settings) (*Result, error) {
	if len(train) == 0 {
		return nil, fmt.Errorf("optimizer requires a non-empty train set")
	}

	rng := rand.New(rand.NewSource(settings.Seed))

	_, seedScores := evaluateVector(ctx, deps.TaskModel, j, seedPrompt, train, settings)
	seed := Candidate{ID: 0, Prompt: seedPrompt, ParentID: -1, Scores: seedScores, Mean: mean(seedScores)}

	result := &Result{
		SeedPrompt:     seedPrompt,
		BestPrompt:     seed.Prompt,
		BestTrainScore: seed.Mean,
		Pool:           []Candidate{seed},
	}
	notify(settings.OnUpdate, result)

	nextID := 1
	consecutiveRejects := 0
	for round := 1; round <= settings.Iterations; round++ {
		parentID := selectParent(result.Pool, rng)
		parent := findCandidate(result.Pool, parentID)

		minibatch := sampleMinibatch(train, settings.MinibatchSize, rng)

		priorJudged, priorScore := EvaluateOnSet(ctx, deps.TaskModel, j, parent.Prompt, minibatch, settings)
		worst := worstExamples(priorJudged, worstK)

		revisedPrompt, analysis, err := Reflect(ctx, deps.ReflectionModel, parent.Prompt, worst, settings.Retries)

		rec := IterationRecord{Round: round, ParentID: parentID, PriorScore: priorScore, AcceptedID: -1, WorstExamples: worst}
		accepted := false
		if err != nil {
			rec.CandidatePrompt = parent.Prompt
			rec.CandidateScore = priorScore
			rec.Analysis = fmt.Sprintf("reflection error: %v", err)
		} else {
			_, candidateScore := EvaluateOnSet(ctx, deps.TaskModel, j, revisedPrompt, minibatch, settings)
			accepted = candidateScore > priorScore
			rec.CandidatePrompt = revisedPrompt
			rec.CandidateScore = candidateScore
			rec.Analysis = analysis
		}
		rec.Accepted = accepted

		if accepted {
			_, fullScores := evaluateVector(ctx, deps.TaskModel, j, revisedPrompt, train, settings)
			newCandidate := Candidate{
				ID: nextID, Prompt: revisedPrompt, ParentID: parentID, Round: round,
				Scores: fullScores, Mean: mean(fullScores),
			}
			result.Pool = append(result.Pool, newCandidate)
			rec.AcceptedID = newCandidate.ID
			nextID++
			consecutiveRejects = 0

			if best := argmaxMean(result.Pool); best.Mean > result.BestTrainScore {
				result.BestPrompt = best.Prompt
				result.BestTrainScore = best.Mean
			}
		} else {
			consecutiveRejects++
		}

		result.History = append(result.History, rec)
		notify(settings.OnUpdate, result)

		if settings.Patience > 0 && consecutiveRejects >= settings.Patience {
			break
		}
	}

	best := argmaxMean(result.Pool)
	result.BestPrompt = best.Prompt
	result.BestTrainScore = best.Mean
	return result, nil
}

// EvaluateOnSet runs prompt against examples and judges every output,
// returning the per-example verdicts plus the mean Overall score. A
// per-example execution or judging failure is recorded as a worst-possible
// verdict (Overall=0) rather than aborting the whole evaluation - one bad
// example shouldn't crash a batch of otherwise-fine ones.
func EvaluateOnSet(ctx context.Context, taskModel model.ToolCallingChatModel, j *judge.Judge, prompt string, examples []dataset.Example, settings Settings) ([]JudgedExample, float64) {
	concurrency := settings.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	execResults := executor.Run(ctx, taskModel, prompt, examples, concurrency, settings.Retries)

	judged := make([]JudgedExample, len(execResults))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var done int64
	for i, res := range execResults {
		wg.Add(1)
		go func(idx int, r executor.Result) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() {
				utils.Logger().Info("judged example",
					zap.Int64("done", atomic.AddInt64(&done, 1)), zap.Int("total", len(execResults)),
					zap.String("example_id", r.Example.ID), zap.Float64("score", judged[idx].Verdict.Overall))
			}()

			if r.Err != nil {
				judged[idx] = JudgedExample{Example: r.Example, Verdict: failedVerdict(r.Err)}
				return
			}
			v, err := j.Score(ctx, r.Example, r.Output)
			if err != nil {
				judged[idx] = JudgedExample{Example: r.Example, Output: r.Output, Verdict: failedVerdict(err)}
				return
			}
			judged[idx] = JudgedExample{Example: r.Example, Output: r.Output, Verdict: v}
		}(i, res)
	}
	wg.Wait()

	if len(judged) == 0 {
		return judged, 0
	}
	var sum float64
	for _, je := range judged {
		sum += je.Verdict.Overall
	}
	return judged, sum / float64(len(judged))
}

// evaluateVector runs EvaluateOnSet and extracts the per-instance Overall
// score vector, in the same order as examples - the shape Candidate.Scores
// needs for Pareto comparison. Reuses EvaluateOnSet as-is.
func evaluateVector(ctx context.Context, taskModel model.ToolCallingChatModel, j *judge.Judge, prompt string, examples []dataset.Example, settings Settings) ([]JudgedExample, []float64) {
	judged, _ := EvaluateOnSet(ctx, taskModel, j, prompt, examples, settings)
	scores := make([]float64, len(judged))
	for i, je := range judged {
		scores[i] = je.Verdict.Overall
	}
	return judged, scores
}

// mean returns the arithmetic mean of scores, or 0 for an empty slice.
func mean(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// findCandidate returns the pool candidate with the given ID. id always
// comes from selectParent or a prior append to the same pool, so a miss
// indicates a programmer error, not bad input.
func findCandidate(pool []Candidate, id int) Candidate {
	for _, c := range pool {
		if c.ID == id {
			return c
		}
	}
	panic(fmt.Sprintf("optimizer: candidate id %d not found in pool", id))
}

// argmaxMean returns the pool candidate with the highest Mean score,
// breaking ties by whichever is found first (lowest ID, since pool is
// append-ordered).
func argmaxMean(pool []Candidate) Candidate {
	best := pool[0]
	for _, c := range pool[1:] {
		if c.Mean > best.Mean {
			best = c
		}
	}
	return best
}

// notify invokes fn with result if fn is set. Result is not mutated again
// until the caller (Run's loop) proceeds, so fn can safely read/serialize it.
func notify(fn func(*Result), result *Result) {
	if fn != nil {
		fn(result)
	}
}

func failedVerdict(err error) *judge.Verdict {
	return &judge.Verdict{Overall: 0, Pass: false, Feedback: fmt.Sprintf("execution/judge failed: %v", err)}
}

// sampleMinibatch returns a random subset of train of the given size
// (or the whole set, if size is non-positive or at least as large as train).
func sampleMinibatch(train []dataset.Example, size int, rng *rand.Rand) []dataset.Example {
	if size <= 0 || size >= len(train) {
		return train
	}
	shuffled := append([]dataset.Example(nil), train...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled[:size]
}

// worstExamples returns the k lowest-scoring judged examples.
func worstExamples(judged []JudgedExample, k int) []JudgedExample {
	sorted := append([]JudgedExample(nil), judged...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Verdict.Overall < sorted[j].Verdict.Overall
	})
	if k > len(sorted) {
		k = len(sorted)
	}
	return sorted[:k]
}
