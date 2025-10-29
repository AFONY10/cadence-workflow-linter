package testdata

import (
	"time"

	"go.uber.org/cadence/workflow"
)

func Helper2() time.Time {
	return time.Now() // should now be flagged
}

func MyWorkflow(ctx workflow.Context) {
	_ = Helper2()
}
