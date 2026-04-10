package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tailscale/hujson"
)

type JSONCValidator struct{}

var _ Validator = JSONCValidator{}

func (JSONCValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := hujson.Parse(b)
	if err != nil {
		line, col := parseHujsonError(err)
		if line > 0 {
			return false, &ValidationError{
				Err:    fmt.Errorf("error at line %v column %v: %w", line, col, err),
				Line:   line,
				Column: col,
			}
		}
		return false, err
	}
	return true, nil
}

func (JSONCValidator) MarshalToJSON(b []byte) ([]byte, error) {
	standardized, err := hujson.Standardize(b)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(standardized, &raw); err != nil {
		return nil, err
	}
	if doc, ok := raw.(map[string]any); ok {
		delete(doc, "$schema")
		return json.Marshal(doc)
	}
	return standardized, nil
}

func (JSONCValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	standardized, err := hujson.Standardize(b)
	if err != nil {
		return false, err
	}

	var raw any
	if err := json.Unmarshal(standardized, &raw); err != nil {
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

	delete(doc, "$schema")
	cleanDoc, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return JSONSchemaValidate(schemaURL, cleanDoc)
}

// parseHujsonError extracts line and column from hujson error messages.
// hujson errors look like: "line 3, column 5: ..."
func parseHujsonError(err error) (line int, col int) {
	msg := err.Error()
	if n, _ := fmt.Sscanf(msg, "line %d, column %d", &line, &col); n >= 1 {
		return line, col
	}
	if strings.Contains(msg, "line ") {
		if n, _ := fmt.Sscanf(msg, "line %d", &line); n >= 1 {
			return line, 0
		}
	}
	return 0, 0
}
