package validator

import (
	"bytes"

	"howett.net/plist"
)

// PlistValidator is used to validate a byte slice that is intended to represent a
// Apple Property List file (plist).
type PlistValidator struct{}

// Validate checks if the provided byte slice represents a valid .plist file.
func (PlistValidator) Validate(b []byte) (bool, error) {
	var output interface{}
	plistDecoder := plist.NewDecoder(bytes.NewReader(b))
	err := plistDecoder.Decode(&output)
	if err != nil {
		return false, err
	}
	return true, nil
}
