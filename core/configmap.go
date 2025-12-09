package core

import (
	"strings"

	"github.com/afony10/cadence-workflow-linter/config"
)

// ApplyConfigOverrides updates issue severities and messages based on the
// provided configuration RuleSet. It matches by Rule ID and applies any
// configured severity or message. Messages may include the placeholder
// %FUNC% which will be replaced with the issue.Callee (or Func if Callee is empty).
func ApplyConfigOverrides(issues []Issue, rs *config.RuleSet) []Issue {
	if rs == nil {
		return issues
	}

	// Build overrides map from the unified schema
	type ruleOverride struct {
		severity string
		message  string
	}
	overrides := make(map[string]ruleOverride)

	for ruleID, rule := range rs.Rules {
		overrides[ruleID] = ruleOverride{
			severity: rule.DefaultSeverity,
			message:  rule.Message,
		}
	}

	// Apply overrides
	out := make([]Issue, 0, len(issues))
	for _, iss := range issues {
		if ov, ok := overrides[iss.Rule]; ok {
			if ov.severity != "" {
				iss.Severity = ov.severity
			}
			if ov.message != "" {
				// Prefer explicit Callee when provided by detectors, fall back to the
				// canonical Func name otherwise. This allows detectors to supply the
				// precise API function name for message substitution.
				fn := iss.Callee
				if fn == "" {
					fn = iss.Func
				}
				msg := strings.ReplaceAll(ov.message, "%FUNC%", fn)
				iss.Message = msg
			}
		}
		out = append(out, iss)
	}
	return out
}
