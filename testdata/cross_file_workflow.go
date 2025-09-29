package testcrossfile

import (
	"go.uber.org/cadence/workflow"
)

// Workflow that calls helper from another file
func CrossFileWorkflow(ctx workflow.Context) error {
	// This should trigger a violation in CrossFileHelper
	timestamp := CrossFileHelper()
	_ = timestamp
	return nil
}