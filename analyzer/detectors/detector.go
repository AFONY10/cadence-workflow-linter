package detectors // This creates the detectors package

import (
	"go/ast"
	"go/token"
)

type Detector interface {
	Name() string
	Detect(node ast.Node, fset *token.FileSet) []Issue
}

type Issue struct {
	Message string
	Line    int
	Column  int
}
