package main

import (
	"fmt"
	"os"

	"github.com/afony10/cadence-workflow-linter/analyzer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cadence-workflow-linter <file.go>")
		os.Exit(1)
	}

	filename := os.Args[1]
	analyzer.RunAnalysis(filename)
}
