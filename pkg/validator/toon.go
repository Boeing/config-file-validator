package validator

import (
	"github.com/toon-format/toon-go"
)

type ToonValidator struct{}

var _ Validator = ToonValidator{}

// ValidateSyntax implements the Validator interface by attempting to
// unmarshall a byte array of toon
func (ToonValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := toon.Decode(b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (ToonValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
