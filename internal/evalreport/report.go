// Package evalreport is the only package that touches the validation set.
// It runs the seed prompt and the optimizer's winning candidate against the
// frozen val set exactly once each, compares them, and renders a report.
package evalreport

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"github.com/Conversly/prompt-opt/internal/dataset"
	"github.com/Conversly/prompt-opt/internal/judge"
	"github.com/Conversly/prompt-opt/internal/optimizer"
)

// gapThreshold is how much higher the best candidate's train score can be
// than its val score before it's flagged as possible overfitting.
const gapThreshold = 0.10

// ExampleScore is one val example's score under both prompts.
type ExampleScore struct {
	ID        string  `json:"id"`
	Category  string  `json:"category,omitempty"`
	SeedScore float64 `json:"seed_score"`
	BestScore float64 `json:"best_score"`
	SeedPass  bool    `json:"seed_pass"`
	BestPass  bool    `json:"best_pass"`
}

// Comparison is the seed-vs-best result on the frozen validation set.
type Comparison struct {
	SeedAggregate      float64            `json:"seed_aggregate"`
	BestAggregate      float64            `json:"best_aggregate"`
	Delta              float64            `json:"delta"`
	SeedByCategory     map[string]float64 `json:"seed_by_category,omitempty"`
	BestByCategory     map[string]float64 `json:"best_by_category,omitempty"`
	BestTrainScore     float64            `json:"best_train_score"`
	BestValScore       float64            `json:"best_val_score"`
	TrainValGapWarning bool               `json:"train_val_gap_warning"`
	PerExample         []ExampleScore     `json:"per_example"`
}

// Evaluate scores both seedPrompt and bestPrompt against val (which must
// never have been used during search) and compares them. bestTrainScore is
// the optimizer's own final train-set score for bestPrompt, used only to
// compute the train/val gap warning.
func Evaluate(ctx context.Context, taskModel model.ToolCallingChatModel, j *judge.Judge, seedPrompt, bestPrompt string, val []dataset.Example, bestTrainScore float64, settings optimizer.Settings) (*Comparison, error) {
	if len(val) == 0 {
		return nil, fmt.Errorf("validation set is empty - dataset too small for the requested --val-split, or every example landed in the train split")
	}

	seedJudged, seedAgg := optimizer.EvaluateOnSet(ctx, taskModel, j, seedPrompt, val, settings)
	bestJudged, bestAgg := optimizer.EvaluateOnSet(ctx, taskModel, j, bestPrompt, val, settings)

	cmp := &Comparison{
		SeedAggregate:  seedAgg,
		BestAggregate:  bestAgg,
		Delta:          bestAgg - seedAgg,
		BestTrainScore: bestTrainScore,
		BestValScore:   bestAgg,
		SeedByCategory: aggregateByCategory(seedJudged),
		BestByCategory: aggregateByCategory(bestJudged),
		PerExample:     mergePerExample(seedJudged, bestJudged),
	}
	cmp.TrainValGapWarning = (bestTrainScore - bestAgg) > gapThreshold

	return cmp, nil
}

func aggregateByCategory(judged []optimizer.JudgedExample) map[string]float64 {
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, je := range judged {
		cat := je.Example.Category
		sums[cat] += je.Verdict.Overall
		counts[cat]++
	}
	out := make(map[string]float64, len(sums))
	for cat, sum := range sums {
		out[cat] = sum / float64(counts[cat])
	}
	return out
}

// mergePerExample zips seed and best results by index. Both come from
// EvaluateOnSet called with the same val slice, so they're guaranteed to be
// the same length and in the same example order.
func mergePerExample(seedJudged, bestJudged []optimizer.JudgedExample) []ExampleScore {
	out := make([]ExampleScore, 0, len(seedJudged))
	for i := range seedJudged {
		s, b := seedJudged[i], bestJudged[i]
		out = append(out, ExampleScore{
			ID:        s.Example.ID,
			Category:  s.Example.Category,
			SeedScore: s.Verdict.Overall,
			BestScore: b.Verdict.Overall,
			SeedPass:  s.Verdict.Pass,
			BestPass:  b.Verdict.Pass,
		})
	}
	return out
}

// RenderMarkdown produces a human-readable summary: headline numbers, an
// overfitting warning if warranted, per-category breakdown (if the dataset
// used categories), the round-by-round reflection analysis timeline, and
// the best candidate's best/worst val examples.
func RenderMarkdown(result *optimizer.Result, cmp *Comparison) string {
	var b strings.Builder

	b.WriteString("# Prompt Optimization Report\n\n")
	b.WriteString(fmt.Sprintf("- Rounds run: `%d`\n", len(result.History)))
	b.WriteString(fmt.Sprintf("- Seed val score: `%.3f`\n", cmp.SeedAggregate))
	b.WriteString(fmt.Sprintf("- Best val score: `%.3f`\n", cmp.BestAggregate))
	b.WriteString(fmt.Sprintf("- Delta: `%+.3f`\n", cmp.Delta))
	b.WriteString(fmt.Sprintf("- Best candidate's final train score: `%.3f`\n\n", cmp.BestTrainScore))

	if cmp.TrainValGapWarning {
		b.WriteString(fmt.Sprintf(
			"> **Warning**: train score (%.3f) is notably higher than val score (%.3f) - possible overfitting to the search set.\n\n",
			cmp.BestTrainScore, cmp.BestValScore,
		))
	}

	if hasRealCategories(cmp.SeedByCategory) {
		b.WriteString("## Per-category val scores\n\n")
		b.WriteString("| Category | Seed | Best | Delta |\n|---|---|---|---|\n")
		for _, cat := range sortedKeys(cmp.SeedByCategory) {
			label := cat
			if label == "" {
				label = "(uncategorized)"
			}
			seedScore, bestScore := cmp.SeedByCategory[cat], cmp.BestByCategory[cat]
			b.WriteString(fmt.Sprintf("| %s | %.3f | %.3f | %+.3f |\n", label, seedScore, bestScore, bestScore-seedScore))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Round-by-round analysis\n\n")
	if len(result.History) == 0 {
		b.WriteString("_No rounds were run._\n\n")
	}
	for _, rec := range result.History {
		status := "rejected"
		if rec.Accepted {
			status = "accepted"
		}
		b.WriteString(fmt.Sprintf("### Round %d - %s\n\n", rec.Round, status))
		b.WriteString(fmt.Sprintf("- Minibatch score: `%.3f` -> `%.3f`\n", rec.PriorScore, rec.CandidateScore))
		b.WriteString(fmt.Sprintf("- Analysis: %s\n\n", rec.Analysis))
		writeWorstExamples(&b, rec.WorstExamples)
	}

	worst, best := bestWorstExamples(cmp.PerExample, 3)
	b.WriteString("## Best candidate's worst val examples\n\n")
	writeExampleTable(&b, worst)
	b.WriteString("\n## Best candidate's best val examples\n\n")
	writeExampleTable(&b, best)

	return b.String()
}

func hasRealCategories(byCategory map[string]float64) bool {
	for cat := range byCategory {
		if cat != "" {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func bestWorstExamples(scores []ExampleScore, n int) (worst, best []ExampleScore) {
	sorted := append([]ExampleScore(nil), scores...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].BestScore < sorted[j].BestScore })

	if n > len(sorted) {
		n = len(sorted)
	}
	worst = sorted[:n]

	best = append([]ExampleScore(nil), sorted[len(sorted)-n:]...)
	sort.Slice(best, func(i, j int) bool { return best[i].BestScore > best[j].BestScore })
	return worst, best
}

// writeWorstExamples renders the judge's per-example scores and feedback for
// the examples reflection was shown - the evidence behind each round's
// Analysis line, not just the aggregate score movement.
func writeWorstExamples(b *strings.Builder, worst []optimizer.JudgedExample) {
	if len(worst) == 0 {
		return
	}
	b.WriteString("**Worst examples shown to reflection:**\n\n")
	for _, je := range worst {
		cat := je.Example.Category
		if cat == "" {
			cat = "-"
		}
		b.WriteString(fmt.Sprintf("- **%s** (%s) — overall `%.2f`\n", je.Example.ID, cat, je.Verdict.Overall))
		if je.Output != "" {
			b.WriteString(fmt.Sprintf("  - Response: %s\n", truncate(je.Output, 300)))
		}
		if je.Verdict.Feedback != "" {
			b.WriteString(fmt.Sprintf("  - Judge feedback: %s\n", truncate(je.Verdict.Feedback, 300)))
		}
	}
	b.WriteString("\n")
}

// truncate shortens s to at most n runes (not bytes, so multi-byte
// characters like em dashes or curly quotes in real model output aren't
// split mid-character) and collapses newlines for single-line rendering.
func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func writeExampleTable(b *strings.Builder, scores []ExampleScore) {
	if len(scores) == 0 {
		b.WriteString("_none_\n")
		return
	}
	b.WriteString("| ID | Category | Seed | Best |\n|---|---|---|---|\n")
	for _, s := range scores {
		cat := s.Category
		if cat == "" {
			cat = "-"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.3f | %.3f |\n", s.ID, cat, s.SeedScore, s.BestScore))
	}
}
