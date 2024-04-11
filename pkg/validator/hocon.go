package validator

import (
	"github.com/gurkankaymak/hocon"
)

// HoconValidator is used to validate a byte slice that is intended to represent a
// HOCON file.
type HoconValidator struct{}

// Validate checks if the provided byte slice represents a valid .hocon file.
func (HoconValidator) Validate(b []byte) (bool, error) {
	_, err := hocon.ParseString(string(b))
	if err != nil {
		return false, err
	}

	return true, nil
}
