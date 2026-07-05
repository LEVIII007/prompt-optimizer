package optimizer

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"

	"github.com/cloudwego/eino/components/model"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/executor"
	"github.com/Conversly/prompt-opt/internal/judge"
)

// worstK is how many of the lowest-scoring minibatch examples get shown to
// the reflection model each round.
const worstK = 3

// Run performs the reflective-mutation search loop against train, starting
// from seedPrompt, and returns the best candidate found plus the full
// iteration history. It never touches a validation set - that comparison
// happens separately, once, after Run returns.
//
// This is greedy hill-climbing (one running "best" candidate), not GEPA's
// full Pareto candidate pool - a deliberate v1 simplification.
func Run(ctx context.Context, deps Deps, j *judge.Judge, seedPrompt string, train []dataset.Example, settings Settings) (*Result, error) {
	if len(train) == 0 {
		return nil, fmt.Errorf("optimizer requires a non-empty train set")
	}

	fullEvalEvery := settings.FullEvalEvery
	if fullEvalEvery < 1 {
		fullEvalEvery = 1
	}

	rng := rand.New(rand.NewSource(settings.Seed))

	best := seedPrompt
	_, bestTrainScore := EvaluateOnSet(ctx, deps.TaskModel, j, best, train, settings)

	result := &Result{SeedPrompt: seedPrompt, BestPrompt: best, BestTrainScore: bestTrainScore}

	roundsSinceImprovement := 0
	for round := 1; round <= settings.Iterations; round++ {
		minibatch := sampleMinibatch(train, settings.MinibatchSize, rng)

		priorJudged, priorScore := EvaluateOnSet(ctx, deps.TaskModel, j, best, minibatch, settings)
		worst := worstExamples(priorJudged, worstK)

		revisedPrompt, analysis, err := Reflect(ctx, deps.ReflectionModel, best, worst, settings.Retries)
		if err != nil {
			result.History = append(result.History, IterationRecord{
				Round:           round,
				CandidatePrompt: best,
				PriorScore:      priorScore,
				CandidateScore:  priorScore,
				Accepted:        false,
				Analysis:        fmt.Sprintf("reflection error: %v", err),
				WorstExamples:   worst,
			})
			continue
		}

		_, candidateScore := EvaluateOnSet(ctx, deps.TaskModel, j, revisedPrompt, minibatch, settings)
		accepted := candidateScore > priorScore

		result.History = append(result.History, IterationRecord{
			Round:           round,
			CandidatePrompt: revisedPrompt,
			PriorScore:      priorScore,
			CandidateScore:  candidateScore,
			Accepted:        accepted,
			Analysis:        analysis,
			WorstExamples:   worst,
		})

		if accepted {
			best = revisedPrompt
		}

		if round%fullEvalEvery == 0 || round == settings.Iterations {
			_, fullScore := EvaluateOnSet(ctx, deps.TaskModel, j, best, train, settings)
			if fullScore > bestTrainScore {
				bestTrainScore = fullScore
				roundsSinceImprovement = 0
			} else {
				roundsSinceImprovement++
			}
			if roundsSinceImprovement >= settings.Patience {
				break
			}
		}
	}

	result.BestPrompt = best
	result.BestTrainScore = bestTrainScore
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
	for i, res := range execResults {
		wg.Add(1)
		go func(idx int, r executor.Result) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

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
