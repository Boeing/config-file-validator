package gojust

import "fmt"

type ParseError struct {
	Pos     Position
	Message string
	File    string
	Err     error // underlying error, if any
}

func (e *ParseError) Error() string {
	file := e.File
	if file == "" {
		file = "<input>"
	}
	return fmt.Sprintf("%s:%d:%d: %s", file, e.Pos.Line, e.Pos.Column, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

type Diagnostic struct {
	Pos      Position
	Severity Severity
	Message  string
	File     string
}

func (d Diagnostic) String() string {
	file := d.File
	if file == "" {
		file = "<input>"
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", file, d.Pos.Line, d.Pos.Column, d.Severity, d.Message)
}
