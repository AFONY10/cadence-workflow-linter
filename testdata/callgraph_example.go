package testdata

import (
	"context"
	"time"

	"go.uber.org/cadence/workflow"
)

// This is a workflow function (has workflow.Context parameter)
func MyWorkflow(ctx workflow.Context, input string) error {
	// Direct function call: MyWorkflow -> helperFunction
	result := helperFunction(input)

	// Another direct call: MyWorkflow -> processData
	processData(result)

	// Call to time.Now (external package call - not tracked in CallGraph)
	_ = time.Now()

	return nil
}

// This is a helper function that will be called by workflow
func helperFunction(data string) string {
	// Direct call: helperFunction -> formatData
	return formatData(data + "_processed")
}

// Another helper function
func formatData(input string) string {
	// Direct call: formatData -> validateInput
	if validateInput(input) {
		return input + "_formatted"
	}
	return input
}

// Validation function
func validateInput(s string) bool {
	return len(s) > 0
}

// This is an activity function (has context.Context parameter)
func MyActivity(ctx context.Context, data string) (string, error) {
	// Activity calls helper: MyActivity -> activityHelper
	return activityHelper(data), nil
}

// Helper for activity
func activityHelper(data string) string {
	// Even though this calls time.Now, it won't be flagged
	// because it's reachable from activity, not workflow
	_ = time.Now()
	return data + "_activity_result"
}

// Standalone function (not workflow or activity)
func standaloneFunction() {
	// This won't be tracked in workflow reachability
	_ = time.Now()
}
