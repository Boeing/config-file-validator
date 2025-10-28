package validator

import (
	"encoding/xml"
)

type XMLValidator struct{}

var _ Validator = XMLValidator{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of xml
func (XMLValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := xml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (XMLValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
