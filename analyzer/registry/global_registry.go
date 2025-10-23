package registry

import (
	"strings"
	"sync"
)

var (
	globalMu sync.RWMutex
	globalWR *WorkflowRegistry
)

// SetGlobal sets the package-global WorkflowRegistry used by analyzers/vettool.
func SetGlobal(wr *WorkflowRegistry) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalWR = wr
}

// Global returns the package-global WorkflowRegistry previously set (or nil).
func Global() *WorkflowRegistry {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalWR
}

// Canonical builds a canonical symbol name for a function: "pkg/path.Func".
// If pkgPath is empty, "local" is used.
func Canonical(pkgPath, funcName string) string {
	p := strings.TrimSpace(pkgPath)
	if p == "" {
		p = "local"
	}
	return p + "." + funcName
}
