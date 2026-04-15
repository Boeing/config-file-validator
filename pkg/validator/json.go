package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// JSONValidator validates JSON files. When ForbidDuplicateKeys is true,
// duplicate keys in objects are reported as errors.
type JSONValidator struct {
	ForbidDuplicateKeys bool
}

var _ Validator = JSONValidator{}

func (v JSONValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		var synErr *json.SyntaxError
		if errors.As(err, &synErr) {
			offset := int(synErr.Offset)
			line := 1 + strings.Count(string(b)[:offset], "\n")
			column := 1 + offset - (strings.LastIndex(string(b)[:offset], "\n") + len("\n"))
			return false, &ValidationError{
				Err:    synErr,
				Line:   line,
				Column: column,
			}
		}
		return false, err
	}

	if v.ForbidDuplicateKeys {
		if err := checkJSONDuplicateKeys(b); err != nil {
			return false, err
		}
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

	delete(doc, "$schema")
	cleanDoc, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return JSONSchemaValidate(schemaURL, cleanDoc)
}

// checkJSONDuplicateKeys walks the JSON token stream and reports duplicate keys.
func checkJSONDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	return checkDuplicateKeysInDecoder(dec)
}

func checkDuplicateKeysInDecoder(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return nil
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}

	if delim == '{' {
		return checkDuplicateKeysInObject(dec)
	}
	if delim == '[' {
		return checkDuplicateKeysInArray(dec)
	}
	return nil
}

func checkDuplicateKeysInObject(dec *json.Decoder) error {
	seen := make(map[string]struct{})
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		key, ok := tok.(string)
		if !ok {
			continue
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = struct{}{}

		// Consume the value — recurse to check nested objects
		if err := checkDuplicateKeysInDecoder(dec); err != nil {
			return err
		}
	}
	// consume closing }
	_, _ = dec.Token()
	return nil
}

func checkDuplicateKeysInArray(dec *json.Decoder) error {
	for dec.More() {
		if err := checkDuplicateKeysInDecoder(dec); err != nil {
			return err
		}
	}
	// consume closing ]
	_, _ = dec.Token()
	return nil
}
