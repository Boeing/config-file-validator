package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type JSONValidator struct{}

var _ Validator = JSONValidator{}

func getCustomErr(input []byte, err error) error {
	var jsonError *json.SyntaxError
	if !errors.As(err, &jsonError) {
		return err
	}

	offset := int(jsonError.Offset)
	line := 1 + strings.Count(string(input)[:offset], "\n")
	column := 1 + offset - (strings.LastIndex(string(input)[:offset], "\n") + len("\n"))
	return fmt.Errorf("error at line %v column %v: %w", line, column, jsonError)
}

func (JSONValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		customError := getCustomErr(b, err)
		return false, customError
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
	if !ok || schemaURL == "" {
		return true, ErrNoSchema
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
