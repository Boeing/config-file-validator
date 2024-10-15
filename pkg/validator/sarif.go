package validator

import (
	"github.com/owenrumney/go-sarif/v2/sarif"
)

type SarifValidator struct{}

// Validate implements the Validator interface by attempting to
// parse .sarif file using UnMarshal.json
func (SarifValidator) Validate(b []byte) (bool, error) {
	_, err := sarif.FromBytes(b)
	if err != nil {
		return false, err
	}
	return true, nil
}
