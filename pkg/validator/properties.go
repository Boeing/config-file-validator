package validator

import (
	"github.com/magiconair/properties"
)

type PropValidator struct{}

var _ Validator = PropValidator{}

// Validate implements the Validator interface by attempting to
// parse a byte array of properties
func (PropValidator) ValidateSyntax(b []byte) (bool, error) {
	l := &properties.Loader{Encoding: properties.UTF8}
	_, err := l.LoadBytes(b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (PropValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
