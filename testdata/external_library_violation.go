package testdata

import (
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// This should trigger external package violations
func ExternalLibraryWorkflow(ctx workflow.Context) error {
	// Should be flagged - UUID generation is non-deterministic
	id := uuid.New()

	// Should be flagged - Redis operations should be in activities
	rdb := redis.NewClient(&redis.Options{})
	value := rdb.Get(ctx, "key")

	// Should NOT be flagged - pkg/errors is in safe list
	err := errors.New("test error")

	// Should NOT be flagged - zap logging is in safe list
	logger := zap.NewNop()
	logger.Info("workflow started")

	return nil
}

// This should NOT trigger violations (not a workflow)
func ExternalLibraryActivity() error {
	// Even though this uses UUID, it's in an activity so shouldn't be flagged
	id := uuid.New()
	return nil
}
