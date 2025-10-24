package validator

import (
	"bytes"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

type EditorConfigValidator struct{}

var _ Validator = EditorConfigValidator{}

// Validate implements the Validator interface by attempting to
// parse a byte array of an editorconfig file using editorconfig-core-go package
func (EditorConfigValidator) ValidateSyntax(b []byte) (bool, error) {
	if _, err := editorconfig.Parse(bytes.NewReader(b)); err != nil {
		return false, err
	}
	return true, nil
}

func (EditorConfigValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
