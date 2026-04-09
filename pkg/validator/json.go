package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type JSONValidator struct{}

var _ Validator = JSONValidator{}

func (JSONValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		var synErr *json.SyntaxError
		if errors.As(err, &synErr) {
			offset := int(synErr.Offset)
			line := 1 + strings.Count(string(b)[:offset], "\n")
			column := 1 + offset - (strings.LastIndex(string(b)[:offset], "\n") + len("\n"))
			return false, &ValidationError{
				Err:    fmt.Errorf("error at line %v column %v: %w", line, column, synErr),
				Line:   line,
				Column: column,
			}
		}
		return false, err
	}
	return true, nil
}

func (JSONValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	if doc, ok := raw.(map[string]any); ok {
		delete(doc, "$schema")
		return json.Marshal(doc)
	}
	return b, nil
}

func (JSONValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
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

	schemaURL = resolveSchemaURL(schemaURL, filePath)

	// Remove $schema from document before validation — it's metadata, not content
	delete(doc, "$schema")
	cleanDoc, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return JSONSchemaValidate(schemaURL, cleanDoc)
}
