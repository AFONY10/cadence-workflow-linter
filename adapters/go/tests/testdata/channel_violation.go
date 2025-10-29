package testdata

import (
	"go.uber.org/cadence/workflow"
)

func ChannelWorkflow(ctx workflow.Context) error {
	ch := make(chan int)
	_ = ch
	return nil
}
