package rubric

import (
	"fmt"

	"github.com/Conversly/prompt-opt/internal/utils"
)

// Load reads a rubric JSON file and validates it.
func Load(path string) (*Rubric, error) {
	var r Rubric
	if err := utils.ReadJSON(path, &r); err != nil {
		return nil, fmt.Errorf("failed to load rubric from %s: %w", path, err)
	}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rubric at %s: %w", path, err)
	}
	return &r, nil
}

// Validate checks structural invariants: at least one dimension, unique
// names, positive scale/weight, and a pass threshold within 0..1.
func (r *Rubric) Validate() error {
	if len(r.Dimensions) == 0 {
		return fmt.Errorf("rubric must have at least one dimension")
	}
	seen := make(map[string]bool, len(r.Dimensions))
	for _, d := range r.Dimensions {
		if d.Name == "" {
			return fmt.Errorf("dimension missing name")
		}
		if seen[d.Name] {
			return fmt.Errorf("duplicate dimension name %q", d.Name)
		}
		seen[d.Name] = true
		if d.Scale <= 0 {
			return fmt.Errorf("dimension %q must have scale > 0", d.Name)
		}
		if d.Weight <= 0 {
			return fmt.Errorf("dimension %q must have weight > 0", d.Name)
		}
	}
	if r.PassThreshold < 0 || r.PassThreshold > 1 {
		return fmt.Errorf("pass_threshold must be between 0 and 1, got %v", r.PassThreshold)
	}
	return nil
}
