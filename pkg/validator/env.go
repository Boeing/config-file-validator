package validator

import (
	"bytes"
	"errors"

	"github.com/hashicorp/go-envparse"
)

type EnvValidator struct{}

var _ Validator = EnvValidator{}

// Validate implements the Validator interface by attempting to
// parse a byte array of a env file using envparse package
func (EnvValidator) ValidateSyntax(b []byte) (bool, error) {
	r := bytes.NewReader(b)
	_, err := envparse.Parse(r)
	if err != nil {
		var customError *envparse.ParseError
		if errors.As(err, &customError) {
			return false, &ValidationError{
				Err:  customError.Err,
				Line: customError.Line,
			}
		}
		return false, err
	}
	return true, nil
}
