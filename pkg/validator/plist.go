package validator

import (
	"bytes"

	"howett.net/plist"
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
		return false, err
	}
	return true, nil
}

func (PlistValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
