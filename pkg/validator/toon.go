package validator

import (
	"encoding/json"

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
	if !ok || schemaURL == "" {
		return true, ErrNoSchema
	}

	delete(doc, "$schema")
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return jsonSchemaValidate(resolveSchemaURL(schemaURL, filePath), docJSON)
}
