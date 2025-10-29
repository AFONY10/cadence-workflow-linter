module github.com/afony10/cadence-workflow-linter

go 1.25.1

require (
	golang.org/x/tools v0.38.0
	gopkg.in/yaml.v3 v3.0.1
)

// Replace for local test module used in adapter testdata. This maps the
// example.com/linttest module used by test fixtures into the nested testdata
// module so the Go toolchain can resolve imports while running tests.
replace example.com/linttest => ./adapters/go/tests/testdata/mod

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)
