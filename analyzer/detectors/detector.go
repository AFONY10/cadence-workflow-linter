package detectors

import (
	"go/token"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type Issue struct {
	File     string `json:"file" yaml:"file"`
	Line     int    `json:"line" yaml:"line"`
	Column   int    `json:"column" yaml:"column"`
	Rule     string `json:"rule" yaml:"rule"`
	Severity string `json:"severity" yaml:"severity"`
	Message  string `json:"message" yaml:"message"`
}

type WorkflowAware interface {
	SetWorkflowRegistry(reg *registry.WorkflowRegistry)
}

type FileContext struct {
	File      string
	Fset      *token.FileSet
	ImportMap map[string]string // alias -> import path (e.g. "rand" -> "math/rand")
}

type FileContextAware interface {
	SetFileContext(ctx FileContext)
}

type IssueProvider interface {
	Issues() []Issue
}
