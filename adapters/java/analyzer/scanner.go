package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/afony10/cadence-workflow-linter/config"
	"github.com/afony10/cadence-workflow-linter/core"
)

// matchInfo represents a detected method call match
type matchInfo struct {
	line    int
	callee  string
	pattern javaMethodPattern
}

// javaMethodPattern represents a pattern extracted from the unified rules schema
type javaMethodPattern struct {
	ruleID   string
	pkg      string
	types    []string
	methods  []string
	severity string
	message  string
}

// MethodInfo represents a single method in a Java file (used by scanner.go)
type MethodInfo struct {
	Name       string
	StartLine  int
	EndLine    int
	BodyLines  []string
	Matches    map[string][]matchInfo // ruleID -> matched calls
	IsWorkflow bool
	IsActivity bool
	Calls      map[string]bool
}

// MethodInfoMultiFile represents a method in multi-file analysis (used by scan_files.go)
type MethodInfoMultiFile struct {
	Key        string
	File       string
	Name       string
	StartLine  int
	EndLine    int
	BodyLines  []string
	Matches    map[string][]matchInfo
	IsWorkflow bool
	Calls      map[string][]int
}

// buildJavaRulePatterns extracts Java method_calls from the unified schema
func buildJavaRulePatterns(rules *config.RuleSet) []javaMethodPattern {
	var patterns []javaMethodPattern

	for ruleID, rule := range rules.Rules {
		javaLang, hasJava := rule.Languages["java"]
		if !hasJava {
			continue
		}

		for _, mc := range javaLang.MethodCalls {
			// Determine severity and message
			severity := mc.Severity
			if severity == "" {
				severity = rule.DefaultSeverity
			}
			message := mc.Message
			if message == "" {
				message = rule.Message
			}

			patterns = append(patterns, javaMethodPattern{
				ruleID:   ruleID,
				pkg:      mc.Package,
				types:    mc.Types,
				methods:  mc.Methods,
				severity: severity,
				message:  message,
			})
		}
	}

	return patterns
}

// matchesPattern checks if a method call matches a Java pattern
func matchesPattern(pkg, typeName, methodName string, pattern javaMethodPattern) bool {
	// Check package match
	if pattern.pkg != "" && !strings.Contains(pkg, pattern.pkg) {
		return false
	}

	// Check type match (empty means any type)
	if len(pattern.types) > 0 {
		typeMatches := false
		for _, t := range pattern.types {
			if typeName == t || strings.Contains(typeName, t) {
				typeMatches = true
				break
			}
		}
		if !typeMatches {
			return false
		}
	}

	// Check method match
	for _, m := range pattern.methods {
		if methodName == m {
			return true
		}
	}

	return false
}

// ScanDirectory walks the given directory (or single file path) and returns
// issues found in Java source files. This is intentionally simple: it looks
// for uses of `Instant.now(` and `System.currentTimeMillis(` as examples of
// time-related APIs that would be nondeterministic in workflows.
// ScanDirectory scans Java files under `root` using the provided rules.
// The rules parameter is the shared, language-agnostic RuleSet loaded by the CLI
// so top-level configuration changes (messages, severities) apply to Java findings.
func ScanDirectory(root string, rules *config.RuleSet) ([]core.Issue, error) {

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return scanFile(root, rules)
	}

	// Collect all .java files and run a project-scoped analysis
	var files []string
	err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi == nil || fi.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".java" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	return scanFiles(files, rules)
}

// ScanFile scans a single Java file and returns any found issues, using the shared RuleSet.
func ScanFile(path string, rules *config.RuleSet) ([]core.Issue, error) {
	return scanFile(path, rules)
}

// Common regex patterns for Java code analysis
var (
	reMethodCall = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]*(?:\.[A-Z][A-Za-z0-9_]*)*?)\.([a-z][A-Za-z0-9_]*)\s*\(`)
	reNew        = regexp.MustCompile(`\bnew\s+([A-Z][A-Za-z0-9_]*)\s*\(`)
)

// scanFile analyzes a single file using the provided RuleSet.
func scanFile(path string, rules *config.RuleSet) ([]core.Issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read whole file into memory for multi-pass analysis
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)

	// Split into lines for accurate line numbers
	lines := strings.Split(content, "\n")

	// Determine if the file contains any workflow indicators (annotations, class names, package)
	fileHasWorkflowIndicator := strings.Contains(content, "@WorkflowMethod") || strings.Contains(content, "@WorkflowInterface") || strings.Contains(content, "implements Workflow") || strings.Contains(strings.ToLower(content), "class") && strings.Contains(content, "Workflow") || strings.Contains(strings.ToLower(content), "package") && strings.Contains(strings.ToLower(content), "workflow")

	// Build Java patterns from unified schema
	javaPatterns := buildJavaRulePatterns(rules)

	// Parse methods with a simple state machine: capture annotations, method signatures and bodies
	methodSigRe := regexp.MustCompile(`^\s*(?:public|protected|private|static|final|synchronized|\s)*[\w\<\>\[\]]+\s+([A-Za-z0-9_]+)\s*\([^\)]*\)\s*\{?\s*$`)
	callRe := regexp.MustCompile(`\b([A-Za-z0-9_]+)\s*\(`)

	methods := make(map[string]*MethodInfo)
	var className string
	var packageName string

	// Capture class and package
	for idx, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "package ") {
			packageName = strings.TrimPrefix(t, "package ")
			packageName = strings.TrimSpace(strings.TrimSuffix(packageName, ";"))
		}
		if strings.HasPrefix(t, "class ") || strings.Contains(t, " class ") {
			// crude class name extraction
			parts := strings.Fields(t)
			for i, p := range parts {
				if p == "class" && i+1 < len(parts) {
					className = strings.Trim(parts[i+1], "{ ")
					break
				}
			}
		}
		// stop early if both found
		if packageName != "" && className != "" {
			break
		}
		_ = idx
	}

	// Second pass: find methods
	lastAnnotation := ""
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "@") {
			lastAnnotation = t
			continue
		}
		// method signature
		if matches := methodSigRe.FindStringSubmatch(t); matches != nil {
			name := matches[1]
			mi := &MethodInfo{
				Name:      name,
				StartLine: i + 1,
				Calls:     make(map[string]bool),
			}
			// heuristics for workflow/activity
			if strings.Contains(lastAnnotation, "@WorkflowMethod") || strings.Contains(lastAnnotation, "@WorkflowInterface") {
				mi.IsWorkflow = true
			}
			if strings.Contains(lastAnnotation, "@ActivityMethod") {
				mi.IsActivity = true
			}
			// class/package level heuristics
			if className != "" && strings.Contains(strings.ToLower(className), "workflow") {
				mi.IsWorkflow = true
			}
			if className != "" && strings.Contains(strings.ToLower(className), "activity") {
				mi.IsActivity = true
			}
			if packageName != "" && strings.Contains(strings.ToLower(packageName), "workflow") {
				mi.IsWorkflow = true
			}

			// gather body until matching braces
			braceDepth := 0
			// if signature line contains '{', start depth at 1
			if strings.Contains(t, "{") {
				braceDepth = 1
			}
			bodyLines := []string{}
			j := i
			for ; j < len(lines); j++ {
				l := lines[j]
				bodyLines = append(bodyLines, l)
				braceDepth += strings.Count(l, "{")
				braceDepth -= strings.Count(l, "}")
				if braceDepth <= 0 && j > i {
					break
				}
			}
			mi.BodyLines = bodyLines
			mi.EndLine = j + 1

			// scan body for configured rule patterns and calls
			mi.Matches = make(map[string][]matchInfo)
			for k, bl := range bodyLines {
				absLine := i + k + 1

				// Check Java method call patterns from unified schema
				for _, pattern := range javaPatterns {
					// Check for method calls: Type.method() or object.method()
					if matches := reMethodCall.FindAllStringSubmatch(bl, -1); matches != nil {
						for _, m := range matches {
							if len(m) >= 3 {
								typePart := m[1]
								methodName := m[2]

								// Extract the simple type name from qualified name
								typeSegments := strings.Split(typePart, ".")
								simpleType := typeSegments[len(typeSegments)-1]

								if matchesPattern(pattern.pkg, simpleType, methodName, pattern) {
									callee := fmt.Sprintf("%s.%s.%s", pattern.pkg, simpleType, methodName)
									mi.Matches[pattern.ruleID] = append(mi.Matches[pattern.ruleID], matchInfo{
										line:    absLine,
										callee:  callee,
										pattern: pattern,
									})
								}
							}
						}
					}

					// Check for constructor calls: new Type()
					if matches := reNew.FindAllStringSubmatch(bl, -1); matches != nil {
						for _, m := range matches {
							if len(m) >= 2 {
								typeName := m[1]
								// Check if pattern is looking for constructor calls
								for _, methodName := range pattern.methods {
									if strings.EqualFold(methodName, "new") || strings.EqualFold(methodName, typeName) {
										if matchesPattern(pattern.pkg, typeName, methodName, pattern) {
											callee := fmt.Sprintf("%s.%s", pattern.pkg, typeName)
											mi.Matches[pattern.ruleID] = append(mi.Matches[pattern.ruleID], matchInfo{
												line:    absLine,
												callee:  callee,
												pattern: pattern,
											})
										}
									}
								}
							}
						}
					}
				}

				// find bare calls for call graph
				for _, cm := range callRe.FindAllStringSubmatch(bl, -1) {
					if len(cm) > 1 {
						callee := cm[1]
						// ignore common language constructs
						if callee == "if" || callee == "for" || callee == "switch" || callee == "return" || callee == "new" || callee == "throw" {
							continue
						}
						mi.Calls[callee] = true
					}
				}
			}

			methods[mi.Name] = mi
			// advance i to end of method
			i = j
			lastAnnotation = ""
		}
	}

	// If file has no workflow indicators and no method marked as workflow, skip reporting
	hasWorkflowMethod := false
	for _, m := range methods {
		if m.IsWorkflow {
			hasWorkflowMethod = true
			break
		}
	}
	if !fileHasWorkflowIndicator && !hasWorkflowMethod {
		// nothing to do
		return nil, nil
	}

	// Build call graph and compute reachability from workflow methods
	callGraph := make(map[string]map[string]bool)
	for name, m := range methods {
		if _, ok := callGraph[name]; !ok {
			callGraph[name] = make(map[string]bool)
		}
		for cal := range m.Calls {
			// only consider calls to known methods in this file
			if _, exists := methods[cal]; exists {
				callGraph[name][cal] = true
			}
		}
	}

	// Find all methods reachable from any workflow method
	reachable := make(map[string]bool)
	queue := []string{}
	for name, m := range methods {
		if m.IsWorkflow {
			reachable[name] = true
			queue = append(queue, name)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for cal := range callGraph[cur] {
			if !reachable[cal] {
				reachable[cal] = true
				queue = append(queue, cal)
			}
		}
	}

	var issues []core.Issue
	// Report occurrences for each rule ID matched in reachable methods
	for name, m := range methods {
		if !reachable[name] {
			continue
		}
		for ruleID, matches := range m.Matches {
			for _, match := range matches {
				issues = append(issues, core.Issue{
					File:     path,
					Line:     match.line,
					Rule:     ruleID,
					Severity: match.pattern.severity,
					Message:  strings.ReplaceAll(match.pattern.message, "%FUNC%", match.callee),
					Func:     name,
					Callee:   match.callee,
				})
			}
		}
	}

	return issues, nil
}
