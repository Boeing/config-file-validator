package validator

import (
	"github.com/editorconfig/editorconfig-core-go/v2"
)

type EditorConfigValidator struct{}

// Validate implements the Validator interface by attempting to
// parse a byte array of an editorconfig file using editorconfig-core-go package
func (EditorConfigValidator) Validate(b []byte) (bool, error) {
	if _, err := editorconfig.ParseBytes(b); err != nil {
		return false, err
	}
	return true, nil
}
