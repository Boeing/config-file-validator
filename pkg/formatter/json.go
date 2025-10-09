package formatter

import (
	"encoding/json"
)

type JSONFormatter struct{}

const INDENT = "  "

// Format implements the formatter interface by attempting to
// unmarshall a byte array of json
func (JSONFormatter) Format(b []byte) ([]byte, error) {
	var output any
	err := json.Unmarshal(b, &output)
	if err != nil {
		return b, err
	}
	outputBytes, err := json.MarshalIndent(output, "", INDENT)
	if err != nil {
		return b, err
	}
	return outputBytes, nil
}
