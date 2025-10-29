package testdata

import (
	"fmt"
	"os"

	"go.uber.org/cadence/workflow"
)

func OutsideWorkflow() {
	fmt.Println("hello")      // should NOT be flagged
	_, _ = os.Open("tmp.txt") // should NOT be flagged
}

func IOInsideWorkflow(ctx workflow.Context) error {
	fmt.Println("hi from wf") // should be flagged
	_, _ = os.Open("tmp.txt") // should be flagged
	return nil
}
