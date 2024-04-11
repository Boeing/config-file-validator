package validator

import (
	"encoding/xml"
)

type XMLValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of xml
func (XMLValidator) Validate(b []byte) (bool, error) {
	var output any
	err := xml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}
