package testdata

import (
	"time"

	"go.uber.org/cadence/workflow"
)

func ValidActivity() {
	_ = time.Now() // should NOT be flagged
}

func MyWorkflow(ctx workflow.Context) error {
	_ = time.Now() // should be flagged
	return nil
}
