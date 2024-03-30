package validator

import (
	"bytes"
	"errors"
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
		var customError *envparse.ParseError
		if errors.As(err, &customError) {
			// we can wrap some useful information with the error
			err = fmt.Errorf("Error at line %v: %w", customError.Line, customError.Err)
		}
		return false, err
	}
	return true, nil
}
