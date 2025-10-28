package validator

import (
	"gopkg.in/ini.v1"
)

type IniValidator struct{}

var _ Validator = IniValidator{}

// Validate implements the Validator interface by attempting to
// parse a byte array of ini
func (IniValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := ini.LoadSources(ini.LoadOptions{}, b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (IniValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
