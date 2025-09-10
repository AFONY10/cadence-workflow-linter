# Cadence Static Analyzer (Go)

This is a prototype CLI tool that performs static analysis on Go Cadence workflow code. Its purpose is to detect potentially non-deterministic code that could break workflow replay or versioning.

## Usage

```bash
go run main.go ./testdata/bad_workflow.go
