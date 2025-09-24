# Cadence Static Analyzer (Go)

This is a prototype CLI tool that performs static analysis on Go Cadence workflow code. Its purpose is to detect potentially non-deterministic code that could break workflow replay or versioning.

## Usage
Run following command from root of repository: (format, json)
```bash
go run . --rules config/rules.yaml --format json /path/to/test/folder
```

If you want to get the output in yml-format, you can run this:
```bash
go run . --rules config/rules.yaml --format yml /path/to/test/folder
```
