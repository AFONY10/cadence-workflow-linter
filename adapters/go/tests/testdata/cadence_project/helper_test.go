package testdata

import (
	"time"

	"go.uber.org/cadence/workflow"
)

func Helper1() time.Time {
	return time.Now() // should now be flagged
}

func Helper2() time.Time {
	return Helper1() // should be flagged as well
}

func MyWorkflow(ctx workflow.Context) {
	_ = Helper2()
}
