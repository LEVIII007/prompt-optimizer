package optimizer

import "math/rand"

// paretoFrontier finds, for every instance in the tracking set, which pool
// candidate(s) achieve the max score on it (ties included), and returns the
// union of those candidates plus each one's win count - the number of
// instances it was tied for best on. All pool candidates must have Scores
// of the same length (the tracking set size).
func paretoFrontier(pool []Candidate) (frontier []int, winCounts map[int]int) {
	winCounts = make(map[int]int)
	n := len(pool[0].Scores)
	for instance := 0; instance < n; instance++ {
		best := pool[0].Scores[instance]
		for _, c := range pool[1:] {
			if c.Scores[instance] > best {
				best = c.Scores[instance]
			}
		}
		for _, c := range pool {
			if c.Scores[instance] == best {
				winCounts[c.ID]++
			}
		}
	}
	frontier = make([]int, 0, len(winCounts))
	for id := range winCounts {
		frontier = append(frontier, id)
	}
	return frontier, winCounts
}

// strictlyDominates reports whether a dominates b: a scores at least as well
// as b on every instance, and strictly better on at least one. a and b must
// be the same length.
func strictlyDominates(a, b []float64) bool {
	betterSomewhere := false
	for i := range a {
		if a[i] < b[i] {
			return false
		}
		if a[i] > b[i] {
			betterSomewhere = true
		}
	}
	return betterSomewhere
}

// pruneDominated drops any frontier candidate that is strictly dominated by
// some other pool member, so a candidate that never uniquely earns its spot
// on the frontier doesn't dilute the selection weighting below.
func pruneDominated(pool []Candidate, frontier []int) []int {
	byID := make(map[int]Candidate, len(pool))
	for _, c := range pool {
		byID[c.ID] = c
	}
	survivors := make([]int, 0, len(frontier))
	for _, id := range frontier {
		candidate := byID[id]
		dominated := false
		for _, other := range pool {
			if other.ID == id {
				continue
			}
			if strictlyDominates(other.Scores, candidate.Scores) {
				dominated = true
				break
			}
		}
		if !dominated {
			survivors = append(survivors, id)
		}
	}
	return survivors
}

// selectParent implements GEPA's Pareto-frontier-weighted sampling: build
// the frontier, prune strictly-dominated members, then sample a survivor
// with probability proportional to its (pre-pruning) win count - candidates
// that are uniquely best on more of the tracking set are more likely to seed
// the next mutation, but every survivor keeps a nonzero chance.
func selectParent(pool []Candidate, rng *rand.Rand) int {
	frontier, winCounts := paretoFrontier(pool)
	survivors := pruneDominated(pool, frontier)
	if len(survivors) == 0 {
		survivors = frontier
	}

	totalWeight := 0
	for _, id := range survivors {
		totalWeight += winCounts[id]
	}
	if totalWeight <= 0 {
		return survivors[rng.Intn(len(survivors))]
	}

	target := rng.Intn(totalWeight)
	cumulative := 0
	for _, id := range survivors {
		cumulative += winCounts[id]
		if target < cumulative {
			return id
		}
	}
	return survivors[len(survivors)-1]
}
