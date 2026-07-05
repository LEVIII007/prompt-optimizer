package optimizer

import (
	"math/rand"
	"testing"
)

func TestParetoFrontierTiesIncludeAllWinners(t *testing.T) {
	pool := []Candidate{
		{ID: 0, Scores: []float64{1.0, 0.5}},
		{ID: 1, Scores: []float64{1.0, 0.3}},
	}

	frontier, winCounts := paretoFrontier(pool)

	if len(frontier) != 2 {
		t.Fatalf("expected both candidates on the frontier (tied on instance 0), got %v", frontier)
	}
	if winCounts[0] != 2 {
		t.Fatalf("expected candidate 0 to win both instances, got %d", winCounts[0])
	}
	if winCounts[1] != 1 {
		t.Fatalf("expected candidate 1 to win only the tied instance, got %d", winCounts[1])
	}
}

func TestPruneDominatedRemovesStrictlyWorseCandidate(t *testing.T) {
	pool := []Candidate{
		{ID: 0, Scores: []float64{1.0, 0.5}}, // dominates ID 1
		{ID: 1, Scores: []float64{1.0, 0.3}}, // strictly dominated by ID 0
		{ID: 2, Scores: []float64{0.2, 0.9}}, // wins instance 1 outright, undominated
	}

	frontier, _ := paretoFrontier(pool)
	survivors := pruneDominated(pool, frontier)

	if containsID(survivors, 1) {
		t.Fatalf("expected strictly-dominated candidate 1 to be pruned, survivors: %v", survivors)
	}
	if !containsID(survivors, 0) || !containsID(survivors, 2) {
		t.Fatalf("expected candidates 0 and 2 to survive pruning, got %v", survivors)
	}
}

func TestSelectParentDegeneratesToSeedWithPoolSizeOne(t *testing.T) {
	pool := []Candidate{{ID: 0, Scores: []float64{0.5, 0.7, 0.1}}}
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 10; i++ {
		if got := selectParent(pool, rng); got != 0 {
			t.Fatalf("expected the only pool candidate (ID 0) to always be selected, got %d", got)
		}
	}
}

// TestSelectParentWeightsBySurvivorWinCount: candidate 0 wins 3 of 4
// instances, candidate 1 wins the other 1, and neither dominates the other
// (each loses on the instance the other wins), so both survive pruning and
// selection weight should split ~3:1 in candidate 0's favor. The seed is
// fixed, so this is a deterministic check, not a statistically flaky one -
// the same seed produces the exact same 20000-draw sequence every run.
func TestSelectParentWeightsBySurvivorWinCount(t *testing.T) {
	pool := []Candidate{
		{ID: 0, Scores: []float64{1.0, 1.0, 1.0, 0.0}},
		{ID: 1, Scores: []float64{0.0, 0.0, 0.0, 1.0}},
	}
	rng := rand.New(rand.NewSource(7))

	const trials = 20000
	counts := map[int]int{}
	for i := 0; i < trials; i++ {
		counts[selectParent(pool, rng)]++
	}

	ratio := float64(counts[0]) / float64(trials)
	if ratio < 0.65 || ratio > 0.85 {
		t.Fatalf("expected candidate 0 selected ~75%% of the time (3:1 weight), got %.2f%% (%v)", ratio*100, counts)
	}
}

func containsID(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
