package validator

import (
	"gopkg.in/yaml.v3"
)

type YAMLValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of yaml
func (YAMLValidator) Validate(b []byte) (bool, error) {
	var output any
	err := yaml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}
