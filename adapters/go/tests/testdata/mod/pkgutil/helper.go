package pkgutil

import "time"

// Helper returns the current time (intentionally uses time.Now to be flagged by detectors in tests)
func Helper() time.Time {
	return time.Now()
}

// SafeHelper is a safe helper that doesn't use problematic APIs
func SafeHelper() string {
	return "safe"
}
