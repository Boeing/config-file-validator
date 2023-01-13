package validator

import (
	"errors"
	"fmt"
	"github.com/pelletier/go-toml/v2"
)

type TomlValidator struct{}

func (tv TomlValidator) Validate(b []byte) (bool, error) {
	var output interface{}
	err := toml.Unmarshal(b, &output)
	var derr *toml.DecodeError
	if errors.As(err, &derr) {
		row, col := derr.Position()
		return false, fmt.Errorf("Error at line %v column %v: %v", row, col, err)
	}
	return true, nil
}
