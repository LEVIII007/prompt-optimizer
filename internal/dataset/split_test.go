package dataset

import (
	"fmt"
	"testing"
)

func makeExamples(n int, category string) []Example {
	examples := make([]Example, 0, n)
	for i := 0; i < n; i++ {
		examples = append(examples, Example{
			ID:       fmt.Sprintf("%s-%d", category, i),
			Category: category,
			Input:    "input",
		})
	}
	return examples
}

func TestSplitDeterministicForSameSeed(t *testing.T) {
	examples := append(makeExamples(10, "a"), makeExamples(10, "b")...)

	train1, val1 := Split(examples, 0.3, 42)
	train2, val2 := Split(examples, 0.3, 42)

	if !sameIDs(train1, train2) || !sameIDs(val1, val2) {
		t.Fatalf("expected identical split for same seed, got different results")
	}
}

func TestSplitDiffersForDifferentSeed(t *testing.T) {
	examples := append(makeExamples(20, "a"), makeExamples(20, "b")...)

	_, val1 := Split(examples, 0.3, 1)
	_, val2 := Split(examples, 0.3, 2)

	if sameIDs(val1, val2) {
		t.Fatalf("expected different seeds to (very likely) produce different val sets")
	}
}

func TestSplitStratifiesByCategory(t *testing.T) {
	examples := append(makeExamples(10, "a"), makeExamples(10, "b")...)

	train, val := Split(examples, 0.3, 7)

	if len(train)+len(val) != len(examples) {
		t.Fatalf("expected train+val to cover all examples, got %d+%d != %d", len(train), len(val), len(examples))
	}

	valCounts := countByCategory(val)
	if valCounts["a"] == 0 || valCounts["b"] == 0 {
		t.Fatalf("expected stratified split to include both categories in val, got %+v", valCounts)
	}
	// 30% of 10 rounds to 3 per category.
	if valCounts["a"] != 3 || valCounts["b"] != 3 {
		t.Fatalf("expected 3 val examples per category, got %+v", valCounts)
	}
}

func TestSplitFallsBackToRandomWhenNotAllCategorized(t *testing.T) {
	examples := makeExamples(5, "a")
	examples = append(examples, Example{ID: "no-category", Input: "input"})

	train, val := Split(examples, 0.4, 3)
	if len(train)+len(val) != len(examples) {
		t.Fatalf("expected train+val to cover all examples, got %d+%d != %d", len(train), len(val), len(examples))
	}
}

func TestSplitEdgeRatios(t *testing.T) {
	examples := makeExamples(10, "a")

	train, val := Split(examples, 0, 1)
	if len(train) != 10 || len(val) != 0 {
		t.Fatalf("valRatio=0 expected all train, got train=%d val=%d", len(train), len(val))
	}

	train, val = Split(examples, 1, 1)
	if len(train) != 0 || len(val) != 10 {
		t.Fatalf("valRatio=1 expected all val, got train=%d val=%d", len(train), len(val))
	}
}

func sameIDs(a, b []Example) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}

func countByCategory(examples []Example) map[string]int {
	counts := make(map[string]int)
	for _, ex := range examples {
		counts[ex.Category]++
	}
	return counts
}
