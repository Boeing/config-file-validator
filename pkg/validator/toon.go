package validator

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/toon-format/toon-go"
)

type ToonValidator struct{}

var _ Validator = ToonValidator{}

func (ToonValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := toon.Decode(b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (ToonValidator) MarshalToJSON(b []byte) ([]byte, error) {
	raw, err := toon.Decode(b)
	if err != nil {
		return nil, err
	}
	if doc, ok := raw.(map[string]any); ok {
		delete(doc, "$schema")
		return json.Marshal(doc)
	}
	return json.Marshal(raw)
}

func (ToonValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	raw, err := toon.Decode(b)
	if err != nil {
		return false, err
	}

	doc, ok := raw.(map[string]any)
	if !ok {
		return true, ErrNoSchema
	}

	schemaRef, ok := doc["$schema"]
	if !ok {
		return true, ErrNoSchema
	}

	schemaURL, ok := schemaRef.(string)
	if !ok {
		return false, fmt.Errorf("$schema must be a string, got %T", schemaRef)
	}
	if schemaURL == "" {
		return false, errors.New("$schema must not be empty")
	}

	delete(doc, "$schema")
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return JSONSchemaValidate(resolveSchemaURL(schemaURL, filePath), docJSON)
}
