package testdata

import (
	"go.uber.org/cadence/workflow"
)

func GoroutineWorkflow(ctx workflow.Context) error {
	go func() {
		println("bad goroutine")
	}()
	return nil
}
