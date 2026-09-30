package validator

import (
	"bytes"
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
				Err:    err,
				Line:   line,
				Column: col,
			}
		}
		return false, err
	}
	return true, nil
}

func (JSONCValidator) MarshalToJSON(b []byte) ([]byte, error) {
	standardized, err := hujson.Standardize(bytes.Clone(b))
	if err != nil {
		return nil, err
	}
	return standardized, nil
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
