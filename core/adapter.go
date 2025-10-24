package core

// Adapter is a minimal interface for language adapters (linters).
// An adapter analyzes the target (file or directory) and returns a list of Issues.
// This small abstraction makes it easy to add other language adapters later.
type Adapter interface {
	Analyze(target string) ([]Issue, error)
}
