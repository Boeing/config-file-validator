package validator

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/gurkankaymak/hocon"
)

var hoconPosRe = regexp.MustCompile(`(.*?)\s*at: (\d+):(\d+),?\s*(.*)`)

// HoconValidator is used to validate a byte slice that is intended to represent a
// HOCON file.
type HoconValidator struct{}

var _ Validator = HoconValidator{}

// Validate checks if the provided byte slice represents a valid .hocon file.
func (HoconValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := hocon.ParseString(string(b))
	if err != nil {
		if m := hoconPosRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ := strconv.Atoi(m[2])
			col, _ := strconv.Atoi(m[3])
			msg := strings.TrimSpace(m[1])
			if m[4] != "" {
				if msg != "" {
					msg += ": "
				}
				msg += m[4]
			}
			return false, &ValidationError{Err: errors.New(msg), Line: line, Column: col}
		}
		return false, err
	}

	return true, nil
}
