package dataset

import (
	"fmt"

	"github.com/Conversly/prompt-opt/internal/utils"
)

// Load reads a dataset JSON file (an array of Example) and validates that
// every example has a unique, non-empty id and a non-empty input.
func Load(path string) ([]Example, error) {
	var examples []Example
	if err := utils.ReadJSON(path, &examples); err != nil {
		return nil, fmt.Errorf("failed to load dataset from %s: %w", path, err)
	}
	if len(examples) == 0 {
		return nil, fmt.Errorf("dataset at %s is empty", path)
	}

	seen := make(map[string]bool, len(examples))
	for i, ex := range examples {
		if ex.ID == "" {
			return nil, fmt.Errorf("example at index %d is missing an id", i)
		}
		if seen[ex.ID] {
			return nil, fmt.Errorf("duplicate example id %q", ex.ID)
		}
		seen[ex.ID] = true
		if ex.Input == "" {
			return nil, fmt.Errorf("example %q is missing input", ex.ID)
		}
	}
	return examples, nil
}
