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

// scanFiles implements a project-scoped analysis across multiple Java files.
func scanFiles(paths []string, rules *config.RuleSet) ([]core.Issue, error) {
	// Prepare Java-specific patterns for known rule IDs. Prefer precompiled
	// mappings if set by the caller; otherwise compile from rules.LanguageMappings.
	javaRulePatterns := map[string][]*regexp.Regexp{}
	if precompiledMappings != nil && len(precompiledMappings) > 0 {
		for ruleID, regs := range precompiledMappings {
			javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], regs...)
		}
	} else if rules != nil && rules.LanguageMappings != nil {
		if jm, ok := rules.LanguageMappings["java"]; ok {
			for ruleID, pats := range jm {
				for _, p := range pats {
					if re, err := regexp.Compile(p); err == nil {
						javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], re)
					} else {
						javaRulePatterns[ruleID] = append(javaRulePatterns[ruleID], regexp.MustCompile(regexp.QuoteMeta(p)))
					}
				}
			}
		}
	}
	// fallbacks
	if _, ok := javaRulePatterns["TimeUsage"]; !ok {
		javaRulePatterns["TimeUsage"] = []*regexp.Regexp{reInstant, reSysNow}
	}
	if _, ok := javaRulePatterns["Randomness"]; !ok {
		javaRulePatterns["Randomness"] = []*regexp.Regexp{regexp.MustCompile(`\bnew\s+Random\s*\(`), regexp.MustCompile(`\bThreadLocalRandom\.`), regexp.MustCompile(`\bRandom\.`)}
	}

	type MethodInfo struct {
		Key        string
		File       string
		Name       string
		StartLine  int
		EndLine    int
		BodyLines  []string
		Matches    map[string][]int
		IsWorkflow bool
		Calls      map[string][]int
	}

	methodsByKey := make(map[string]*MethodInfo)
	methodsByName := make(map[string][]string)
	fileHasWorkflow := make(map[string]bool)

	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		content := string(b)
		lines := strings.Split(content, "\n")

		hasWorkflowIndicator := strings.Contains(content, "@WorkflowMethod") || strings.Contains(content, "@WorkflowInterface") || strings.Contains(content, "implements Workflow") || (strings.Contains(strings.ToLower(content), "class") && strings.Contains(content, "Workflow")) || (strings.Contains(strings.ToLower(content), "package") && strings.Contains(strings.ToLower(content), "workflow"))
		fileHasWorkflow[path] = hasWorkflowIndicator

		var className string
		for _, ln := range lines {
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "class ") || strings.Contains(t, " class ") {
				parts := strings.Fields(t)
				for i, p := range parts {
					if p == "class" && i+1 < len(parts) {
						className = strings.Trim(parts[i+1], "{ ")
						break
					}
				}
			}
			if className != "" {
				break
			}
		}

		methodSigRe := regexp.MustCompile(`^\s*(?:public|protected|private|static|final|synchronized|\s)*[\w\<\>\[\]]+\s+([A-Za-z0-9_]+)\s*\([^\)]*\)\s*\{?\s*$`)
		callRe := regexp.MustCompile(`\b([A-Za-z0-9_]+)\s*\(`)
		lastAnnotation := ""
		for i := 0; i < len(lines); i++ {
			ln := lines[i]
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "@") {
				lastAnnotation = t
				continue
			}
			if matches := methodSigRe.FindStringSubmatch(t); matches != nil {
				name := matches[1]
				key := path + "#" + name
				mi := &MethodInfo{
					Key:       key,
					File:      path,
					Name:      name,
					StartLine: i + 1,
					Calls:     make(map[string][]int),
					Matches:   make(map[string][]int),
				}
				if strings.Contains(lastAnnotation, "@WorkflowMethod") || strings.Contains(lastAnnotation, "@WorkflowInterface") {
					mi.IsWorkflow = true
				}
				if hasWorkflowIndicator && (strings.Contains(strings.ToLower(className), "workflow") || strings.Contains(strings.ToLower(path), "workflow")) {
					mi.IsWorkflow = true
				}

				braceDepth := 0
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

				for k, bl := range bodyLines {
					absLine := i + k + 1
					for ruleID, pats := range javaRulePatterns {
						for _, p := range pats {
							if p.MatchString(bl) {
								mi.Matches[ruleID] = append(mi.Matches[ruleID], absLine)
							}
						}
					}
					for _, cm := range callRe.FindAllStringSubmatch(bl, -1) {
						if len(cm) > 1 {
							callee := cm[1]
							if callee == "if" || callee == "for" || callee == "switch" || callee == "return" || callee == "new" || callee == "throw" {
								continue
							}
							mi.Calls[callee] = append(mi.Calls[callee], absLine)
						}
					}
				}

				methodsByKey[key] = mi
				methodsByName[name] = append(methodsByName[name], key)
				i = j
				lastAnnotation = ""
			}
		}
	}

	anyWorkflow := false
	for _, v := range fileHasWorkflow {
		if v {
			anyWorkflow = true
			break
		}
	}
	if !anyWorkflow {
		return nil, nil
	}

	// callGraph maps caller key -> callee key -> list of callsite lines in caller
	callGraph := make(map[string]map[string][]int)
	for k, m := range methodsByKey {
		if _, ok := callGraph[k]; !ok {
			callGraph[k] = make(map[string][]int)
		}
		for cal, callLines := range m.Calls {
			if keys, ok := methodsByName[cal]; ok {
				for _, kk := range keys {
					callGraph[k][kk] = append(callGraph[k][kk], callLines...)
				}
			}
		}
	}

	reachable := make(map[string]bool)
	var queue []string
	workflowRoots := []string{}
	for k, m := range methodsByKey {
		if m.IsWorkflow {
			reachable[k] = true
			queue = append(queue, k)
			workflowRoots = append(workflowRoots, k)
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

	// helper: find shortest path from any workflow root to target using BFS
	type prevInfo struct {
		from     string
		callLine int
	}
	findPath := func(target string) ([]string, map[string]prevInfo) {
		prev := make(map[string]prevInfo)
		q := make([]string, 0)
		seen := make(map[string]bool)
		for _, r := range workflowRoots {
			q = append(q, r)
			seen[r] = true
			prev[r] = prevInfo{"", 0}
		}
		for len(q) > 0 {
			cur := q[0]
			q = q[1:]
			if cur == target {
				// reconstruct
				path := []string{}
				for p := cur; p != ""; p = prev[p].from {
					path = append([]string{p}, path...)
				}
				return path, prev
			}
			for nbr, lines := range callGraph[cur] {
				if !seen[nbr] {
					seen[nbr] = true
					callLine := 0
					if len(lines) > 0 {
						callLine = lines[0]
					}
					prev[nbr] = prevInfo{from: cur, callLine: callLine}
					q = append(q, nbr)
				}
			}
		}
		return nil, nil
	}

	for _, m := range methodsByKey {
		if !reachable[m.Key] {
			continue
		}
		for ruleID, linesMatched := range m.Matches {
			for _, ln := range linesMatched {
				text := ""
				b, err := os.ReadFile(m.File)
				if err == nil {
					fileLines := strings.Split(string(b), "\n")
					if ln-1 < len(fileLines) {
						text = strings.TrimSpace(fileLines[ln-1])
					}
				}
				callee := ""
				if strings.Contains(text, "Instant.now") {
					callee = "java.time.Instant.now"
				} else if strings.Contains(text, "System.currentTimeMillis") {
					callee = "java.lang.System.currentTimeMillis"
				}

				// build human-readable call stack (use call-site lines for callers)
				path, prev := findPath(m.Key)
				callstack := []string{}
				if path != nil {
					for i, k := range path {
						if mi, ok := methodsByKey[k]; ok {
							bn := filepath.Base(mi.File)
							var lineNum int
							if i < len(path)-1 {
								// use the call site line where this method invoked the next
								next := path[i+1]
								if p, ok := prev[next]; ok && p.callLine > 0 {
									lineNum = p.callLine
								} else {
									lineNum = mi.StartLine
								}
							} else {
								// final (reported) method: use exact issue line
								lineNum = ln
							}
							callstack = append(callstack, fmt.Sprintf("%s (%s:%d)", mi.Name, bn, lineNum))
						}
					}
				}

				issues = append(issues, core.Issue{
					File:      m.File,
					Line:      ln,
					Rule:      ruleID,
					Severity:  "warning",
					Message:   "",
					Func:      m.Name,
					Callee:    callee,
					CallStack: callstack,
				})
			}
		}
	}

	return issues, nil
}
