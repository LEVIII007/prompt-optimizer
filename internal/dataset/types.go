package dataset

// Turn is one message in a prior conversation, used to give the task model
// multi-turn context ahead of Example.Input.
type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Example is a single test case: an input the candidate prompt must respond
// to, plus optional context the judge can use to score the response.
type Example struct {
	ID        string `json:"id"`
	Category  string `json:"category,omitempty"`
	History   []Turn `json:"history,omitempty"`
	Input     string `json:"input"`
	Reference string `json:"reference,omitempty"`
	Notes     string `json:"notes,omitempty"`
}
