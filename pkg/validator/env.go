package validator

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/go-envparse"
)

type EnvValidator struct{}

// Validate implements the Validator interface by attempting to
// parse a byte array of a env file using envparse package
func (envv EnvValidator) Validate(b []byte) (bool, error) {
	r := bytes.NewReader(b)
	_, err := envparse.Parse(r)
	if err != nil {
		customError := err.(*envparse.ParseError)
		return false, fmt.Errorf("Error at line %v: %v", customError.Line, customError.Err)
	}
	return true, nil
}
