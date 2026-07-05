package rubric

// Dimension is one scoring axis in a rubric: the judge scores a response
// 0..Scale on it, Weight controls how much it counts toward the aggregate,
// and Required marks it as a hard pass/fail gate independent of the
// weighted average.
type Dimension struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Scale       int     `json:"scale"`
	Weight      float64 `json:"weight"`
	Required    bool    `json:"required"`
}

// Rubric is the full set of dimensions a judge scores a response on, plus
// the minimum weighted-average score (0..1) for a response to "pass".
type Rubric struct {
	Dimensions    []Dimension `json:"dimensions"`
	PassThreshold float64     `json:"pass_threshold"`
}

// TotalWeight sums the weights of all dimensions, used to normalize the
// weighted average into a 0..1 aggregate score.
func (r *Rubric) TotalWeight() float64 {
	var total float64
	for _, d := range r.Dimensions {
		total += d.Weight
	}
	return total
}
