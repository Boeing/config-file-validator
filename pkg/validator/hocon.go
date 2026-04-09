package validator

import (
	"regexp"
	"strconv"

	"github.com/gurkankaymak/hocon"
)

var hoconLineColRe = regexp.MustCompile(`at: (\d+):(\d+)`)

// HoconValidator is used to validate a byte slice that is intended to represent a
// HOCON file.
type HoconValidator struct{}

var _ Validator = HoconValidator{}

// Validate checks if the provided byte slice represents a valid .hocon file.
func (HoconValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := hocon.ParseString(string(b))
	if err != nil {
		if m := hoconLineColRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ := strconv.Atoi(m[1])
			col, _ := strconv.Atoi(m[2])
			return false, &ValidationError{Err: err, Line: line, Column: col}
		}
		return false, err
	}

	return true, nil
}
