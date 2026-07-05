package rubric

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidRubric(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rubric.json")
	content := `{
		"dimensions": [
			{"name": "accuracy", "description": "correct", "scale": 1, "weight": 3, "required": true},
			{"name": "tone", "description": "nice", "scale": 5, "weight": 1, "required": false}
		],
		"pass_threshold": 0.75
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	r, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Dimensions) != 2 {
		t.Fatalf("expected 2 dimensions, got %d", len(r.Dimensions))
	}
	if r.TotalWeight() != 4 {
		t.Fatalf("expected total weight 4, got %v", r.TotalWeight())
	}
}

func TestValidateRejectsEmptyDimensions(t *testing.T) {
	r := Rubric{}
	if err := r.Validate(); err == nil {
		t.Fatalf("expected error for empty dimensions")
	}
}

func TestValidateRejectsDuplicateNames(t *testing.T) {
	r := Rubric{
		Dimensions: []Dimension{
			{Name: "accuracy", Scale: 1, Weight: 1},
			{Name: "accuracy", Scale: 1, Weight: 1},
		},
	}
	if err := r.Validate(); err == nil {
		t.Fatalf("expected error for duplicate dimension names")
	}
}

func TestValidateRejectsBadScaleOrWeight(t *testing.T) {
	cases := []Dimension{
		{Name: "a", Scale: 0, Weight: 1},
		{Name: "a", Scale: 1, Weight: 0},
		{Name: "a", Scale: -1, Weight: 1},
	}
	for _, d := range cases {
		r := Rubric{Dimensions: []Dimension{d}}
		if err := r.Validate(); err == nil {
			t.Fatalf("expected error for dimension %+v", d)
		}
	}
}

func TestValidateRejectsBadPassThreshold(t *testing.T) {
	r := Rubric{
		Dimensions:    []Dimension{{Name: "a", Scale: 1, Weight: 1}},
		PassThreshold: 1.5,
	}
	if err := r.Validate(); err == nil {
		t.Fatalf("expected error for pass_threshold > 1")
	}
}
