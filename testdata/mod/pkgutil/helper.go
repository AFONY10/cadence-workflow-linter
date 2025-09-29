package pkgutil

import "time"

// Helper function that uses time.Now() - should be flagged when called from workflow
func Helper() time.Time {
	return time.Now() // This should trigger a violation when reachable from workflow
}

// Another helper that doesn't violate rules - should not be flagged
func SafeHelper() string {
	return "safe operation"
}
