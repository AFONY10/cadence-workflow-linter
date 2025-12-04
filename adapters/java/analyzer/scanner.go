package analyzer

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/afony10/cadence-workflow-linter/core"
)

// ScanDirectory walks the given directory (or single file path) and returns
// issues found in Java source files. This is intentionally simple: it looks
// for uses of `Instant.now(` and `System.currentTimeMillis(` as examples of
// time-related APIs that would be nondeterministic in workflows.
func ScanDirectory(root string) ([]core.Issue, error) {
	var out []core.Issue

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return scanFile(root)
	}

	err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// propagate
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".java" {
			return nil
		}
		issues, sErr := scanFile(path)
		if sErr != nil {
			return sErr
		}
		out = append(out, issues...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ScanFile scans a single Java file and returns any found issues.
func ScanFile(path string) ([]core.Issue, error) {
	return scanFile(path)
}

var (
	reInstant = regexp.MustCompile(`\bInstant\.now\s*\(`)
	reSysNow  = regexp.MustCompile(`\bSystem\.currentTimeMillis\s*\(`)
)

func scanFile(path string) ([]core.Issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Quick heuristic: only report issues for files that look like workflow files.
	// We check for common Cadence/Temporal Java annotations or naming conventions
	// to avoid flagging ordinary utility Java files.
	// Read the file into memory once to run the heuristic.
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(contentBytes)
	isWorkflow := false
	// common Java Cadence/Temporal annotations and patterns
	if strings.Contains(content, "@WorkflowMethod") || strings.Contains(content, "@WorkflowInterface") || strings.Contains(content, "implements Workflow") || strings.Contains(strings.ToLower(content), "class ") && strings.Contains(content, "Workflow") || strings.Contains(strings.ToLower(content), "package ") && strings.Contains(strings.ToLower(content), "workflow") {
		isWorkflow = true
	}

	if !isWorkflow {
		// don't flag non-workflow Java files by default
		return nil, nil
	}

	var issues []core.Issue
	// We'll scan line-by-line so we can report accurate line numbers.
	r := bufio.NewReader(f)
	lineNum := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		lineNum++

		if reInstant.MatchString(line) {
			issues = append(issues, core.Issue{
				File:     path,
				Line:     lineNum,
				Rule:     "TimeUsage",
				Severity: "warning",
				Message:  "Use of `Instant.now()` which may be nondeterministic in workflows",
				Func:     "Instant.now",
				Callee:   "java.time.Instant.now",
			})
		}
		if reSysNow.MatchString(line) {
			issues = append(issues, core.Issue{
				File:     path,
				Line:     lineNum,
				Rule:     "TimeUsage",
				Severity: "warning",
				Message:  "Use of `System.currentTimeMillis()` which may be nondeterministic in workflows",
				Func:     "System.currentTimeMillis",
				Callee:   "java.lang.System.currentTimeMillis",
			})
		}

		if err == io.EOF {
			break
		}
	}
	return issues, nil
}
