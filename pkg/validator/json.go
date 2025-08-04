package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type JSONValidator struct{}

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
func (JSONValidator) Validate(b []byte) (bool, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		customError := getCustomErr(b, err)
		return false, customError
	}
	return true, nil
}
