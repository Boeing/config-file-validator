package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JsonValidator struct{}

// Returns a custom error message that contains the unmarshal
// error message along with the line and character
// number where the error occurred when parsing the JSON
func getCustomErr(input []byte, err error) error {
	jsonError := err.(*json.SyntaxError)
	offset := int(jsonError.Offset)
	line := 1 + strings.Count(string(input)[:offset], "\n")
	column := 1 + offset - (strings.LastIndex(string(input)[:offset], "\n") + len("\n"))
	return fmt.Errorf("Error at line %v column %v: %v", line, column, jsonError)
}

// Validate implements the Validator interface by attempting to
// unmarshall a byte array of json
func (jv JsonValidator) Validate(b []byte) (bool, error) {
	var output interface{}
	err := json.Unmarshal(b, &output)
	if err != nil {
		customError := getCustomErr(b, err)
		return false, customError
	}
	return true, nil
}
