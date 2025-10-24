package core

// Rule describes a canonical rule in the core rule catalog.
type Rule struct {
	ID       string `json:"id" yaml:"id"`
	Message  string `json:"message" yaml:"message"`
	Severity string `json:"severity" yaml:"severity"` // e.g. error, warning
	Hint     string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

// DefaultCatalog returns a small starter catalog of canonical rules.
// Adapters and detectors should reference these rule IDs when emitting issues.
func DefaultCatalog() map[string]Rule {
	return map[string]Rule{
		"TimeNow": {
			ID:       "TimeNow",
			Message:  "Detected non-deterministic time.Now() call in workflow",
			Severity: "error",
			Hint:     "Use workflow.Now(ctx) or pass time through activities",
		},
		"Random": {
			ID:       "Random",
			Message:  "Detected non-deterministic random call in workflow",
			Severity: "error",
			Hint:     "Use deterministic randomness provided by workflow or activities",
		},
		"IO": {
			ID:       "IO",
			Message:  "Detected IO call in workflow",
			Severity: "warning",
			Hint:     "Perform IO in activities, not in workflows",
		},
		"Goroutine": {
			ID:       "Goroutine",
			Message:  "Detected goroutine spawn in workflow",
			Severity: "error",
			Hint:     "Avoid starting goroutines directly in workflows",
		},
		"MapIteration": {
			ID:       "MapIteration",
			Message:  "Detected map iteration in workflow. Map iteration order in Go is nondeterministic",
			Severity: "warning",
			Hint:     "Sort keys or otherwise ensure deterministic iteration",
		},
		"Network": {
			ID:       "Network",
			Message:  "Detected network call in workflow",
			Severity: "error",
			Hint:     "Perform network calls in activities",
		},
	}
}
