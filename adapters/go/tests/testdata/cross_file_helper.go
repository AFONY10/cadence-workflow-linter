package testcrossfile

import (
	"time"
)

// Helper function in same package that uses time.Now()
func CrossFileHelper() time.Time {
	return time.Now() // This should be flagged when called from workflow
}
