package validator

import (
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

type TomlValidator struct{}

var _ Validator = TomlValidator{}

func (TomlValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := toml.Unmarshal(b, &output)
	var derr *toml.DecodeError
	if errors.As(err, &derr) {
		row, col := derr.Position()
		return false, fmt.Errorf("error at line %v column %v: %w", row, col, err)
	}
	return true, nil
}

func (TomlValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
