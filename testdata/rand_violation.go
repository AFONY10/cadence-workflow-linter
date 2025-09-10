package workflows

import (
	"math/rand"

	"go.uber.org/cadence/workflow"
)

func AnotherWorkflow(ctx workflow.Context) error {
	_ = rand.Intn(10) // Nondeterministic
	return nil
}
