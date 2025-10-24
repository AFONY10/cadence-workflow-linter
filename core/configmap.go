package core

import (
	"strings"

	"github.com/afony10/cadence-workflow-linter/config"
)

// ApplyConfigOverrides updates issue severities and messages based on the
// provided configuration RuleSet. It matches by Rule ID and applies any
// configured severity or message. Messages may include the placeholder
// %FUNC% which will be replaced with the issue.Func value.
func ApplyConfigOverrides(issues []Issue, rs *config.RuleSet) []Issue {
	if rs == nil {
		return issues
	}

	overrides := make(map[string]config.FunctionRule)

	// Function call based rules
	for _, fr := range rs.FunctionCalls {
		if fr.Rule != "" {
			overrides[fr.Rule] = fr
		}
	}

	// Map iteration section (uses FunctionRule shape in YAML)
	for _, mr := range rs.FunctionCalls { // map iteration may be stored in a different place; keep conservative
		if mr.Rule != "" {
			overrides[mr.Rule] = mr
		}
	}

	// Disallowed imports (map by Rule)
	for _, ir := range rs.DisallowedImports {
		if ir.Rule != "" {
			// convert ImportRule -> FunctionRule-like for overrides
			overrides[ir.Rule] = config.FunctionRule{Rule: ir.Rule, Severity: ir.Severity, Message: ir.Message}
		}
	}

	// External package rules
	for _, er := range rs.ExternalPackages {
		if er.Rule != "" {
			overrides[er.Rule] = config.FunctionRule{Rule: er.Rule, Severity: er.Severity, Message: er.Message}
		}
	}

	// Map-specific section (if present in YAML under map_iteration)
	// Note: our loader currently unmarshals into FunctionCalls and others; if
	// map_iteration is present as its own key we fallback to scanning Raw YAML
	// is out of scope here; keep this simple.

	// Apply overrides
	out := make([]Issue, 0, len(issues))
	for _, iss := range issues {
		if ov, ok := overrides[iss.Rule]; ok {
			if ov.Severity != "" {
				iss.Severity = ov.Severity
			}
			if ov.Message != "" {
				// Prefer explicit Callee when provided by detectors, fall back to the
				// canonical Func name otherwise. This allows detectors to supply the
				// precise API function name for message substitution.
				fn := iss.Callee
				if fn == "" {
					fn = iss.Func
				}
				msg := strings.ReplaceAll(ov.Message, "%FUNC%", fn)
				iss.Message = msg
			}
		}
		out = append(out, iss)
	}
	return out
}
