package reporter

// Report holds the results for a single file.
type Report struct {
	// FilePath is the path to the file (as displayed in output).
	FilePath string

	// FileName is the base name of the file.
	FileName string

	// Status indicates the overall outcome for this file.
	Status Status

	// Issues contains all problems found in this file.
	// Empty when Status is StatusPass.
	Issues []Issue

	// Notes are informational messages that don't affect the status.
	// Example: "this file is valid JSONC — use --type-map to validate as JSONC"
	Notes []string

	// IsQuiet suppresses output for this report when true.
	IsQuiet bool
}

// HasErrors reports whether any issue is a failure-level issue (syntax or schema).
func (r Report) HasErrors() bool {
	return r.Status == StatusFail
}

// Status represents the outcome for a single file.
type Status int

const (
	// StatusPass means the file passed all checks (✓).
	StatusPass Status = iota
	// StatusFail means the file has syntax or schema errors (×).
	StatusFail
	// StatusUnformatted means the file is valid but not canonically formatted (~).
	StatusUnformatted
)

// Issue represents a single problem found in a file.
type Issue struct {
	// Type classifies the issue source.
	Type IssueType

	// Message is a human-readable description of the problem.
	// Rendered as-is in stdout output. Should not include the file path or
	// line number — those are added by the reporter.
	Message string

	// Line is the 1-based line number where the issue occurs. 0 if unknown.
	Line int

	// Column is the 1-based column number. 0 if unknown.
	Column int
}

// IssueType classifies the source of an issue.
type IssueType int

const (
	// IssueTypeSyntax is a parse error.
	IssueTypeSyntax IssueType = iota
	// IssueTypeSchema is a schema validation error.
	IssueTypeSchema
	// IssueTypeFormat is a formatting issue (file is valid but not canonical).
	IssueTypeFormat
)

// Reporter is the interface that wraps the Print method.
//
// Print accepts a slice of Report objects and outputs them in the
// reporter's format (stdout, JSON, JUnit, SARIF, GitHub Actions).
type Reporter interface {
	Print(reports []Report) error
}

// GroupNode stores a recursive report grouping tree.
type GroupNode struct {
	Key      string
	Children []*GroupNode
	Reports  []Report
}
