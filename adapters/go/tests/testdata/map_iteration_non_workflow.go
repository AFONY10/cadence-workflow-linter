package testdata

// Simple function with map iteration but NOT a workflow (no workflow.Context)
func NonWorkflowMapIter() {
	m := map[string]int{"x": 1, "y": 2}
	for k := range m {
		_ = k
	}
}
