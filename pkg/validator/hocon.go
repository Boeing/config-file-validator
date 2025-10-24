package validator

import (
	"github.com/gurkankaymak/hocon"
)

// HoconValidator is used to validate a byte slice that is intended to represent a
// HOCON file.
type HoconValidator struct{}

var _ Validator = HoconValidator{}

// Validate checks if the provided byte slice represents a valid .hocon file.
func (HoconValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := hocon.ParseString(string(b))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (HoconValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
