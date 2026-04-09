package validator

import (
	"bytes"
	"regexp"
	"strconv"

	"howett.net/plist"
)

var plistLineColRe = regexp.MustCompile(`at line (\d+) character (\d+)`)

// PlistValidator is used to validate a byte slice that is intended to represent a
// Apple Property List file (plist).
type PlistValidator struct{}

var _ Validator = PlistValidator{}

// Validate checks if the provided byte slice represents a valid .plist file.
func (PlistValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	plistDecoder := plist.NewDecoder(bytes.NewReader(b))
	err := plistDecoder.Decode(&output)
	if err != nil {
		if m := plistLineColRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ := strconv.Atoi(m[1])
			col, _ := strconv.Atoi(m[2])
			return false, &ValidationError{Err: err, Line: line, Column: col}
		}
		return false, err
	}
	return true, nil
}
