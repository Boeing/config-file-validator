package validator

import (
	"encoding/json"
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

func (TomlValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	delete(doc, "$schema")
	return json.Marshal(doc)
}

func (TomlValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return false, err
	}

	schemaRef, ok := doc["$schema"]
	if !ok {
		return true, ErrNoSchema
	}

	schemaURL, ok := schemaRef.(string)
	if !ok || schemaURL == "" {
		return true, ErrNoSchema
	}

	delete(doc, "$schema")
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return JSONSchemaValidate(resolveSchemaURL(schemaURL, filePath), docJSON)
}
