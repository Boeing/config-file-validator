package validator

import (
	"bytes"
	"regexp"
	"strconv"

	kdl "github.com/sblinch/kdl-go"
)

// kdlLineRe matches the "at line N, column M" suffix the upstream parser
// appends to scan/parse errors so we can surface the position via
// ValidationError instead of leaving it buried in the message.
var kdlLineRe = regexp.MustCompile(`at line (\d+), column (\d+)`)

// KdlValidator validates KDL Document Language files via the sblinch/kdl-go
// parser. It implements ValidateSyntax only — KDL has no schema system, so
// it does not implement SchemaValidator.
//
// See https://kdl.dev/ for the KDL spec.
type KdlValidator struct{}

var _ Validator = KdlValidator{}

func (KdlValidator) ValidateSyntax(b []byte) (bool, error) {
	if _, err := kdl.Parse(bytes.NewReader(b)); err != nil {
		if m := kdlLineRe.FindStringSubmatch(err.Error()); m != nil {
			line, lineErr := strconv.Atoi(m[1])
			col, colErr := strconv.Atoi(m[2])
			if lineErr == nil && colErr == nil {
				return false, &ValidationError{Err: err, Line: line, Column: col}
			}
		}
		return false, err
	}
	return true, nil
}
