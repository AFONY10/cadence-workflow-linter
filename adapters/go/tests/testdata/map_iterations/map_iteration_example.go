package testdata

import "go.uber.org/cadence/workflow"

func MapIterWorkflow(ctx workflow.Context) error {
	m := map[string]int{"a": 1, "b": 2}
	for k := range m {
		_ = k
	}
	return nil
}
