package core

import (
	"encoding/json"
)

// Minimal SARIF 2.1 emitter for our issues. This is intentionally small — it
// emits a SARIF document with one run and maps each Issue to a result. This
// is enough for VS Code extensions and other SARIF-aware tools to consume.
func EmitSARIF(issues []Issue) ([]byte, error) {
	// Define simple SARIF structures
	type region struct {
		StartLine   int `json:"startLine,omitempty"`
		StartColumn int `json:"startColumn,omitempty"`
	}
	type physicalLocation struct {
		ArtifactLocation struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region region `json:"region,omitempty"`
	}
	type location struct {
		PhysicalLocation physicalLocation `json:"physicalLocation"`
	}
	type result struct {
		RuleID  string `json:"ruleId,omitempty"`
		Level   string `json:"level,omitempty"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
		Locations []location `json:"locations,omitempty"`
	}

	sarif := map[string]interface{}{
		"version": "2.1.0",
		"$schema": "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json",
		"runs": []map[string]interface{}{
			{
				"tool": map[string]interface{}{
					"driver": map[string]interface{}{
						"name": "cadence-workflow-linter",
					},
				},
				"results": []result{},
			},
		},
	}

	run := sarif["runs"].([]map[string]interface{})[0]
	results := []result{}
	for _, iss := range issues {
		r := result{RuleID: iss.Rule, Level: iss.Severity}
		r.Message.Text = iss.Message
		loc := location{}
		loc.PhysicalLocation.ArtifactLocation.URI = iss.File
		loc.PhysicalLocation.Region.StartLine = iss.Line
		loc.PhysicalLocation.Region.StartColumn = iss.Column
		r.Locations = []location{loc}
		results = append(results, r)
	}
	run["results"] = results

	return json.MarshalIndent(sarif, "", "  ")
}
