package detectors

import (
	"go/ast"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/registry"
	"github.com/afony10/cadence-workflow-linter/config"
)

type importRule struct {
	ruleID   string
	path     string
	severity string
	message  string
}

type ImportDetector struct {
	rules  []importRule
	ctx    FileContext
	wr     *registry.WorkflowRegistry
	issues []Issue
}

func NewImportDetector(ruleSet *config.RuleSet) *ImportDetector {
	var rules []importRule

	// Extract Go-specific import rules from the unified schema
	for ruleID, rule := range ruleSet.Rules {
		goLang, hasGo := rule.Languages["go"]
		if !hasGo {
			continue
		}

		// Process imports
		if goLang.Imports != nil {
			for _, imp := range goLang.Imports.Disallowed {
				rules = append(rules, importRule{
					ruleID:   ruleID,
					path:     imp.Path,
					severity: rule.DefaultSeverity,
					message:  rule.Message,
				})
			}
		}
	}

	return &ImportDetector{rules: rules, issues: []Issue{}}
}

func (d *ImportDetector) SetWorkflowRegistry(reg *registry.WorkflowRegistry) { d.wr = reg }
func (d *ImportDetector) SetFileContext(ctx FileContext)                     { d.ctx = ctx }
func (d *ImportDetector) Issues() []Issue                                    { return d.issues }

func (d *ImportDetector) Visit(node ast.Node) ast.Visitor {
	if len(d.wr.WorkflowFuncs) == 0 {
		return d
	}
	switch n := node.(type) {
	case *ast.ImportSpec:
		path := ""
		if n.Path != nil && len(n.Path.Value) >= 2 {
			path = n.Path.Value[1 : len(n.Path.Value)-1]
		}
		for _, r := range d.rules {
			if r.path == path {
				pos := d.ctx.Fset.Position(n.Pos())
				d.issues = append(d.issues, Issue{
					File:     d.ctx.File,
					Line:     pos.Line,
					Rule:     r.ruleID,
					Severity: r.severity,
					Message:  r.message,
					Func:     "",
				})
			}
		}
	}
	return d
}
