package analyzer

import (
	"os"

	"go/ast"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/core"
)

// GoAdapter is a small wrapper that implements core.Adapter using the
// existing analyzer ScanFile/ScanDirectory functions. This makes the Go
// adapter discoverable and callable by higher-level tooling (like the VS Code
// extension) that expects a core.Adapter.
type GoAdapter struct {
	factory func(*modutils.ModuleInfo) []ast.Visitor
}

// NewGoAdapter creates a GoAdapter. The factory should match the same shape
// used by main.go: func(*modutils.ModuleInfo) []ast.Visitor
func NewGoAdapter(factory func(*modutils.ModuleInfo) []ast.Visitor) *GoAdapter {
	return &GoAdapter{factory: factory}
}

// Analyze runs the analyzer on the given target (file or directory) and
// returns core.Issue slice.
func (ga *GoAdapter) Analyze(target string) ([]core.Issue, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}

	var issues []detectors.Issue
	if info.IsDir() {
		issues, err = ScanDirectory(target, ga.factory)
	} else {
		issues, err = ScanFile(target, ga.factory)
	}
	if err != nil {
		return nil, err
	}

	// detectors.Issue is an alias to core.Issue, so the slice is assignable.
	return issues, nil
}
