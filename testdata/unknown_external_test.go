package testdata

import (
	"github.com/google/uuid"                    // Known bad package
	"github.com/pkg/errors"                     // Known safe package
	mysterylib "github.com/unknown/mystery-lib" // Unknown external package
	"go.uber.org/cadence/workflow"
)

func UnknownExternalWorkflow(ctx workflow.Context) error {
	// Known bad - should be flagged as error
	id := uuid.New()

	// Known safe - should be ignored
	err := errors.New("test")

	// Unknown external - should get info-level warning
	result := mysterylib.DoSomething("data")
	value := mysterylib.ProcessValue(42)

	return nil
}
