package validator

import (
	"encoding/xml"
)

type XmlValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of xml
func (xv XmlValidator) Validate(b []byte) (bool, error) {
	var output interface{}
	err := xml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}
