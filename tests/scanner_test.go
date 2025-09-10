package tests

import (
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/afony10/cadence-workflow-linter/analyzer/detectors"
)

func TestTimeUsageDetector(t *testing.T) {
	fs := token.NewFileSet()
	file, err := os.ReadFile("../testdata/time_violation.go")
	if err != nil {
		t.Fatal(err)
	}

	node, err := parser.ParseFile(fs, "test.go", file, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}

	d := detectors.TimeUsageDetector{}
	issues := d.Detect(node, fs)

	if len(issues) == 0 {
		t.Errorf("Expected at least 1 time usage issue, got 0")
	}
}
