package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type JSONValidator struct{}

var _ Validator = JSONValidator{}

// Returns a custom error message that contains the unmarshal
// error message along with the line and character
// number where the error occurred when parsing the JSON
func getCustomErr(input []byte, err error) error {
	var jsonError *json.SyntaxError
	if !errors.As(err, &jsonError) {
		// not a json.SyntaxError
		// nothing interesting we can wrap into the error
		return err
	}

	offset := int(jsonError.Offset)
	line := 1 + strings.Count(string(input)[:offset], "\n")
	column := 1 + offset - (strings.LastIndex(string(input)[:offset], "\n") + len("\n"))
	return fmt.Errorf("error at line %v column %v: %w", line, column, jsonError)
}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of json
func (JSONValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		customError := getCustomErr(b, err)
		return false, customError
	}
	return true, nil
}

func (JSONValidator) ValidateFormat(b []byte, _ any) (bool, error) {
	var dst bytes.Buffer
	err := json.Indent(&dst, b, "", "  ")
	if err != nil {
		return false, err
	}
	result := bytes.Equal(b, dst.Bytes())
	if !result {
		return false, errors.New("format check failed")
	}
	return true, nil
}
