package validator

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"howett.net/plist"
)

var (
	plistLineColRe     = regexp.MustCompile(`at line (\d+) character (\d+)`)
	plistStripPosition = regexp.MustCompile(`\s*at line \d+ character \d+`)
)

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
		errMsg := err.Error()
		if m := plistLineColRe.FindStringSubmatch(errMsg); m != nil {
			line, _ := strconv.Atoi(m[1])
			col, _ := strconv.Atoi(m[2])
			cleanMsg := plistStripPosition.ReplaceAllString(errMsg, "")
			return false, &ValidationError{Err: fmt.Errorf("%s", strings.TrimSpace(cleanMsg)), Line: line, Column: col}
		}
		return false, err
	}
	return true, nil
}
