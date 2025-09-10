package analyzer

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
)

func RunAnalysis(filename string) {
	fs := token.NewFileSet()

	node, err := parser.ParseFile(fs, filename, nil, parser.AllErrors)
	if err != nil {
		fmt.Printf("Failed to parse file: %v\n", err)
		os.Exit(1)
	}

	activeDetectors := []detectors.Detector{
		detectors.TimeUsageDetector{},
		detectors.RandomnessDetector{},
		detectors.IOCallsDetector{},
	}

	for _, detector := range activeDetectors {
		issues := detector.Detect(node, fs)
		for _, issue := range issues {
			fmt.Printf("%s:%d:%d - %s\n", filename, issue.Line, issue.Column, issue.Message)
		}
	}
}
