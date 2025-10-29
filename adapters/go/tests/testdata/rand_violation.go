package testdata

import (
	"math/rand"

	"go.uber.org/cadence/workflow"
)

func NotAWorkflow() {
	_ = rand.Intn(10) // should NOT be flagged
}

func RandomnessInWorkflow(ctx workflow.Context) error {
	_ = rand.Intn(10) // should be flagged
	return nil
}
