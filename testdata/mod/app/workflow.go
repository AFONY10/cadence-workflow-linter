package app

import (
	"context"
	"example.com/linttest/pkgutil"
	"example.com/linttest/workflow"
)

// Workflow function that calls external package helper - should trigger violation
func TestWorkflow(ctx workflow.Context) error {
	// This call should make pkgutil.Helper reachable from workflow
	// and thus time.Now() in Helper() should be flagged
	timestamp := pkgutil.Helper()
	
	// This call should be fine since SafeHelper doesn't violate rules
	msg := pkgutil.SafeHelper()
	
	_ = timestamp
	_ = msg
	
	return nil
}

// Activity function that calls the same helper - should NOT trigger violation
func TestActivity(ctx context.Context) error {
	timestamp := pkgutil.Helper() // Should be fine in activity
	_ = timestamp
	return nil
}