package utils

import (
	"encoding/json"
	"os"
	"strings"
)

// CleanModelJSON strips markdown code fences that models sometimes wrap
// JSON responses in (```json ... ``` or ``` ... ```) before parsing.
func CleanModelJSON(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

// WriteJSON marshals value as indented JSON and writes it to path.
func WriteJSON(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadJSON reads path and unmarshals it into out.
func ReadJSON(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
