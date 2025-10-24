package detectors

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/core"
)

// Issue is an alias to the canonical core.Issue. This keeps the public API of
// the detectors package stable while consolidating the shared output schema in
// the `core` package for future multi-language adapters.
type Issue = core.Issue

type WorkflowAware interface {
	SetWorkflowRegistry(reg *registry.WorkflowRegistry)
}

type FileContext struct {
	File      string
	Fset      *token.FileSet
	ImportMap map[string]string // alias -> import path
	Node      *ast.File
	TypesInfo *types.Info
}

type FileContextAware interface {
	SetFileContext(ctx FileContext)
}

type PackageAware interface {
	SetPackagePath(pkgPath string)
}

type IssueProvider interface {
	Issues() []Issue
}
