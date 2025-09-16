package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/analyzer/registry"
)

type VisitorFactory func() []ast.Visitor

func buildImportMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := "" // default alias is base of path
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			// base package name (e.g., "math/rand" -> "rand", "go.uber.org/cadence/workflow" -> "workflow")
			slash := strings.LastIndex(path, "/")
			if slash >= 0 {
				alias = path[slash+1:]
			} else {
				alias = path
			}
		}
		m[alias] = path
	}
	return m
}

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

	// Build import alias map
	importMap := buildImportMap(fileNode)

	// Fresh visitors per file
	visitors := factory()

	// Provide contexts
	ctx := detectors.FileContext{
		File:      path,
		Fset:      fset,
		ImportMap: importMap,
	}
	for _, v := range visitors {
		if wa, ok := v.(detectors.WorkflowAware); ok {
			wa.SetWorkflowRegistry(wr)
		}
		if fca, ok := v.(detectors.FileContextAware); ok {
			fca.SetFileContext(ctx)
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
