package detectors

import (
	"go/token"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

// Issue is a structured finding your CLI can serialize.
type Issue struct {
	File    string `json:"file" yaml:"file"`
	Line    int    `json:"line" yaml:"line"`
	Column  int    `json:"column" yaml:"column"`
	Rule    string `json:"rule" yaml:"rule"`
	Message string `json:"message" yaml:"message"`
}

// WorkflowAware detectors need the workflow registry to know
// whether we're inside a workflow function.
type WorkflowAware interface {
	SetWorkflowRegistry(reg *registry.WorkflowRegistry)
}

// FileContextAware detectors need to compute positions (line/col)
// from the file set and include the file path in issues.
type FileContextAware interface {
	SetFileContext(file string, fset *token.FileSet)
}

// IssueProvider exposes collected issues after a Walk.
type IssueProvider interface {
	Issues() []Issue
}
