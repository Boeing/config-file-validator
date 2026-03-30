package validator

import (
	"bytes"
	"errors"

	v210 "github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	v22 "github.com/owenrumney/go-sarif/v3/pkg/report/v22/sarif"
)

type SarifValidator struct{}

type sarifReport interface {
	Validate() error
}

func parseSarif(b []byte) (sarifReport, error) {
	if bytes.Contains(b, []byte(`"version": "2.1.0"`)) {
		return v210.FromBytes(b)
	} else if bytes.Contains(b, []byte(`"version": "2.2"`)) {
		return v22.FromBytes(b)
	}
	return nil, errors.New("unable to determine sarif version")
}

func (SarifValidator) ValidateSchema(b []byte, _ string) (bool, error) {
	report, err := parseSarif(b)
	if err != nil {
		return false, err
	}
	if err := report.Validate(); err != nil {
		return false, err
	}
	return true, nil
}

func (SarifValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := parseSarif(b)
	if err != nil {
		return false, err
	}
	return true, nil
}
