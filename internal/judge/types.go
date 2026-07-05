package judge

// Verdict is the judge's evaluation of a single candidate response. Scores,
// Feedback, and HallucinationFlag come from the judge LLM; Overall and Pass
// are computed in Go from the rubric (never trusted from the LLM directly).
type Verdict struct {
	Scores            map[string]float64 `json:"scores"`
	Feedback          string             `json:"feedback"`
	HallucinationFlag bool               `json:"hallucination_flag"`

	Overall float64 `json:"overall"`
	Pass    bool    `json:"pass"`
}
