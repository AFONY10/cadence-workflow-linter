package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type VisitorFactory func() []ast.Visitor

func ScanFile(path string, factory VisitorFactory) ([]detectors.Issue, error) {
	var all []detectors.Issue

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, path, src, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	// Build workflow registry
	wr := registry.NewWorkflowRegistry()
	ast.Walk(wr, fileNode)

	// Fresh visitors per file
	visitors := factory()

	for _, v := range visitors {
		if wa, ok := v.(detectors.WorkflowAware); ok {
			wa.SetWorkflowRegistry(wr)
		}
		if fca, ok := v.(detectors.FileContextAware); ok {
			fca.SetFileContext(path, fset)
		}
		ast.Walk(v, fileNode)
	}

	for _, v := range visitors {
		if ip, ok := v.(detectors.IssueProvider); ok {
			all = append(all, ip.Issues()...)
		}
	}

	return all, nil
}

func ScanDirectory(root string, factory VisitorFactory) ([]detectors.Issue, error) {
	var all []detectors.Issue
	err := filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
		if info == nil {
			return nil
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		issues, err := ScanFile(path, factory)
		if err == nil {
			all = append(all, issues...)
		}
		return nil
	})
	return all, err
}
