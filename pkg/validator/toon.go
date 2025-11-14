package validator

import (
	"github.com/toon-format/toon-go"
)

type ToonValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of toon
func (ToonValidator) Validate(b []byte) (bool, error) {
	_, err := toon.Decode(b)
    if err != nil {
        return false, err
    }
	return true, nil
}
