package analyzer

// This package is a thin adapter wrapper that forwards to the existing
// analyzer implementation in the repository. It exists so the top-level
// CLI can import a stable adapters path (adapters/go/analyzer) while the
// original analyzer package is gradually moved.

import (
	"go/ast"

	old "github.com/afony10/cadence-workflow-linter/analyzer"
	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/modutils"
)

// Re-export ScanFile and ScanDirectory
func ScanFile(path string, factory func(*modutils.ModuleInfo) []ast.Visitor) ([]detectors.Issue, error) {
	return old.ScanFile(path, factory)
}

func ScanDirectory(root string, factory func(*modutils.ModuleInfo) []ast.Visitor) ([]detectors.Issue, error) {
	return old.ScanDirectory(root, factory)
}
