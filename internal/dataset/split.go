package dataset

import (
	"math"
	"math/rand"
	"sort"
	"strings"
)

// Split partitions examples into train/val sets. valRatio is the fraction
// held out for validation (e.g. 0.3 means ~30% val). If every example has a
// non-empty Category, the split is stratified per category so each category
// keeps roughly the same train/val ratio; otherwise it's a plain shuffle
// split. seed makes the split reproducible across runs with the same input.
func Split(examples []Example, valRatio float64, seed int64) (train, val []Example) {
	if valRatio <= 0 {
		return append([]Example(nil), examples...), nil
	}
	if valRatio >= 1 {
		return nil, append([]Example(nil), examples...)
	}

	rng := rand.New(rand.NewSource(seed))

	if allCategorized(examples) {
		groups := groupByCategory(examples)
		for _, cat := range sortedCategoryKeys(groups) {
			group := append([]Example(nil), groups[cat]...)
			rng.Shuffle(len(group), func(i, j int) { group[i], group[j] = group[j], group[i] })
			cut := valCut(len(group), valRatio)
			val = append(val, group[:cut]...)
			train = append(train, group[cut:]...)
		}
		return train, val
	}

	shuffled := append([]Example(nil), examples...)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	cut := valCut(len(shuffled), valRatio)
	return shuffled[cut:], shuffled[:cut]
}

func allCategorized(examples []Example) bool {
	if len(examples) == 0 {
		return false
	}
	for _, ex := range examples {
		if strings.TrimSpace(ex.Category) == "" {
			return false
		}
	}
	return true
}

func groupByCategory(examples []Example) map[string][]Example {
	groups := make(map[string][]Example)
	for _, ex := range examples {
		groups[ex.Category] = append(groups[ex.Category], ex)
	}
	return groups
}

func sortedCategoryKeys(groups map[string][]Example) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func valCut(n int, ratio float64) int {
	cut := int(math.Round(float64(n) * ratio))
	if cut < 0 {
		cut = 0
	}
	if cut > n {
		cut = n
	}
	return cut
}
