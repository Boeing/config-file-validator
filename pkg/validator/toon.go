package validator

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"

	"github.com/toon-format/toon-go"
)

var toonLineRe = regexp.MustCompile(`line (\d+): (.*)`)

type ToonValidator struct{}

var _ Validator = ToonValidator{}

func (ToonValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := toon.Decode(b)
	if err != nil {
		if m := toonLineRe.FindStringSubmatch(err.Error()); m != nil {
			if line, convErr := strconv.Atoi(m[1]); convErr == nil {
				return false, &ValidationError{Err: errors.New(m[2]), Line: line}
			}
		}
		return false, err
	}
	return true, nil
}

func (ToonValidator) MarshalToJSON(b []byte) ([]byte, error) {
	raw, err := toon.Decode(b)
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}
