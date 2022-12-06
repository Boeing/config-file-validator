package validator

import (
	"gopkg.in/yaml.v3"
)

type YamlValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of yaml
func (yv YamlValidator) Validate(b []byte) (bool, error) {
	var output interface{}
	err := yaml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}
