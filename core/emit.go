package core

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// EmitJSON marshals issues into pretty-printed JSON bytes.
func EmitJSON(issues []Issue) ([]byte, error) {
	return json.MarshalIndent(issues, "", "  ")
}

// EmitYAML marshals issues into YAML bytes.
func EmitYAML(issues []Issue) ([]byte, error) {
	return yaml.Marshal(issues)
}
