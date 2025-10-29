package testdata

import (
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"
)

func MyActivity() {
	// These are VALID inside activities
	fmt.Println("logging from activity")
	_ = time.Now()
	ch := make(chan int)
	_ = ch
	go func() {}()
}

func init() {
	workflow.RegisterActivity(MyActivity)
}
