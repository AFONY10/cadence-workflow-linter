package workflows

import (
	"time"

	"go.uber.org/cadence/workflow"
)

func ExampleWorkflow(ctx workflow.Context) error {
	_ = time.Now() // Not allowed – non-deterministic
	return nil
}
