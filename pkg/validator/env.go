package validator

import (
	"bytes"
	"errors"
	"fmt"

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
			// we can wrap some useful information with the error
			err = fmt.Errorf("error at line %v: %w", customError.Line, customError.Err)
		}
		return false, err
	}
	return true, nil
}

func (EnvValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
