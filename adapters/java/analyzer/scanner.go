package analyzer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/afony10/cadence-workflow-linter/config"
	"github.com/afony10/cadence-workflow-linter/core"
)

// precompiledMappings can be set by the caller (CLI) to avoid recompiling
// regexes in each scanner invocation. Keys are rule IDs.
var precompiledMappings map[string][]*regexp.Regexp

// SetLanguageMappings allows the CLI to supply precompiled regex mappings for Java.
func SetLanguageMappings(m map[string][]*regexp.Regexp) {
	precompiledMappings = m
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

var (
	reInstant = regexp.MustCompile(`\bInstant\.now\s*\(`)
	reSysNow  = regexp.MustCompile(`\bSystem\.currentTimeMillis\s*\(`)
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

	// Prepare Java-specific patterns for known rule IDs. Prefer precompiled
	// mappings if provided by the caller (see SetLanguageMappings), otherwise
	// fall back to mappings present in rules.LanguageMappings.
	javaRulePatterns := map[string][]*regexp.Regexp{}
	if precompiledMappings != nil && len(precompiledMappings) > 0 {
		for ruleID, regs := range precompiledMappings {
			javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], regs...)
		}
	} else if rules != nil && rules.LanguageMappings != nil {
		if jm, ok := rules.LanguageMappings["java"]; ok {
			for ruleID, pats := range jm {
				for _, p := range pats {
					// try compiling as regex; fall back to literal match if invalid
					if re, err := regexp.Compile(p); err == nil {
						javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], re)
					} else {
						javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], regexp.MustCompile(regexp.QuoteMeta(p)))
					}
				}
			}
		}
	}
	// fallbacks if YAML doesn't provide mappings
	if _, ok := javaRulePatterns["TimeUsage"]; !ok {
		javaRulePatterns["TimeUsage"] = []*regexp.Regexp{reInstant, reSysNow}
	}
	if _, ok := javaRulePatterns["Randomness"]; !ok {
		javaRulePatterns["Randomness"] = []*regexp.Regexp{regexp.MustCompile(`\bnew\s+Random\s*\(`), regexp.MustCompile(`\bThreadLocalRandom\.`), regexp.MustCompile(`\bRandom\.`)}
	}

	// Parse methods with a simple state machine: capture annotations, method signatures and bodies
	methodSigRe := regexp.MustCompile(`^\s*(?:public|protected|private|static|final|synchronized|\s)*[\w\<\>\[\]]+\s+([A-Za-z0-9_]+)\s*\([^\)]*\)\s*\{?\s*$`)
	callRe := regexp.MustCompile(`\b([A-Za-z0-9_]+)\s*\(`)

	type MethodInfo struct {
		Name       string
		StartLine  int
		EndLine    int
		BodyLines  []string
		Matches    map[string][]int // ruleID -> line numbers where pattern matched
		IsWorkflow bool
		IsActivity bool
		Calls      map[string]bool
	}

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
			mi.Matches = make(map[string][]int)
			for k, bl := range bodyLines {
				absLine := i + k + 1
				// check each configured Java pattern for rule IDs
				for ruleID, pats := range javaRulePatterns {
					for _, p := range pats {
						if p.MatchString(bl) {
							mi.Matches[ruleID] = append(mi.Matches[ruleID], absLine)
						}
					}
				}
				// find bare calls
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
		for ruleID, linesMatched := range m.Matches {
			for _, ln := range linesMatched {
				// find matched text for Callee hint
				text := ""
				if ln-1 < len(lines) && ln-1 >= 0 {
					text = strings.TrimSpace(lines[ln-1])
				}
				callee := ""
				// heuristics: pick a likely callee token from the line
				if idx := strings.Index(text, "Instant.now"); idx >= 0 {
					callee = "java.time.Instant.now"
				} else if idx := strings.Index(text, "System.currentTimeMillis"); idx >= 0 {
					callee = "java.lang.System.currentTimeMillis"
				}

				issues = append(issues, core.Issue{
					File:     path,
					Line:     ln,
					Rule:     ruleID,
					Severity: "warning",
					Message:  "",
					Func:     name,
					Callee:   callee,
				})
			}
		}
	}

	return issues, nil
}
