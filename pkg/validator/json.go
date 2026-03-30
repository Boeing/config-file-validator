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
