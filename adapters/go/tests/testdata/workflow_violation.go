package testdata

import (
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"
)

func MyWorkflow(ctx workflow.Context) error {
	fmt.Println("bad logging") // should be flagged
	_ = time.Now()             // should be flagged
	go func() {}()             // should be flagged
	ch := make(chan int)       // should be flagged
	_ = ch
	return nil
}

func init() {
	workflow.Register(MyWorkflow)
}
