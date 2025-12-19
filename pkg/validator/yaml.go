package validator

import (
	"gopkg.in/yaml.v3"
)

type YAMLValidator struct{}

var _ Validator = YAMLValidator{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of yaml
func (YAMLValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := yaml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (YAMLValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
