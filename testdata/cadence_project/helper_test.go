package testdata

import (
	"time"

	"go.uber.org/cadence/workflow"
)

func Helper2() {
	time.Now() // should now be flagged
}

func MyWorkflow(ctx workflow.Context) {
	Helper2()
}
