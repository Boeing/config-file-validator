package validator

import (
	"encoding/json"
	"errors"

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
		return false, &ValidationError{
			Err:    err,
			Line:   row,
			Column: col,
		}
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (TomlValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
