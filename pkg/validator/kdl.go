package validator

import (
	"bytes"

	kdl "github.com/sblinch/kdl-go"
)

// KdlValidator validates KDL Document Language files via the sblinch/kdl-go
// parser. It implements ValidateSyntax only — KDL has no schema system, so
// it does not implement SchemaValidator.
//
// See https://kdl.dev/ for the KDL spec.
type KdlValidator struct{}

var _ Validator = KdlValidator{}

func (KdlValidator) ValidateSyntax(b []byte) (bool, error) {
	if _, err := kdl.Parse(bytes.NewReader(b)); err != nil {
		return false, err
	}
	return true, nil
}
