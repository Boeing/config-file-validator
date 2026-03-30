package validator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

type YAMLValidator struct{}

var _ Validator = YAMLValidator{}

func (YAMLValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := yaml.Unmarshal(b, &output)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (YAMLValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	schemaURL := extractYAMLSchemaComment(b)
	if schemaURL == "" {
		return true, ErrNoSchema
	}

	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return false, err
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		return false, err
	}

	return jsonSchemaValidate(resolveSchemaURL(schemaURL, filePath), docJSON)
}

// extractYAMLSchemaComment scans for the yaml-language-server schema modeline:
//
//	# yaml-language-server: $schema=<url>
func extractYAMLSchemaComment(b []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			return ""
		}
		const prefix = "yaml-language-server:"
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(prefix):])
		if after, ok := strings.CutPrefix(rest, "$schema="); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}
