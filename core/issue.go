package core

// Issue is the canonical, language-agnostic representation of a linter finding.
// Adapters (language-specific linters) and the CLI/extension will use this
// shared representation to emit results (JSON/SARIF/etc.).
type Issue struct {
	File      string   `json:"file" yaml:"file"`
	Line      int      `json:"line" yaml:"line"`
	Column    int      `json:"column" yaml:"column"`
	Rule      string   `json:"rule" yaml:"rule"`
	Severity  string   `json:"severity" yaml:"severity"`
	Message   string   `json:"message" yaml:"message"`
	Func      string   `json:"func,omitempty" yaml:"func,omitempty"`
	CallStack []string `json:"callstack,omitempty" yaml:"callstack,omitempty"`
}
