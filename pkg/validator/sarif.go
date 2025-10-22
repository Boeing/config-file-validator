package validator

import (
	"bytes"
	v210 "github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	v22 "github.com/owenrumney/go-sarif/v3/pkg/report/v22/sarif"
)

type SarifValidator struct{}

func (SarifValidator) Validate(b []byte) (bool, error) {
	// Validate syntax
	if bytes.Contains(b, []byte(`"version": "2.1.0"`)) {
		_, err := v210.FromBytes(b)
		if err != nil {
			return false, err
		}
	} else if bytes.Contains(b, []byte(`"version": "2.2"`)) {
		_, err := v22.FromBytes(b)
		if err != nil {
			return false, err
		}
	} else {
		return false, errors.New("Unable to determine sarif version")
	}

	return true, nil
}
