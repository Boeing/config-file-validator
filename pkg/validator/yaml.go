package validator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLValidator validates YAML files.
// Note: yaml.v3 already rejects duplicate keys by default.
type YAMLValidator struct{}

var _ Validator = YAMLValidator{}

var yamlLineRe = regexp.MustCompile(`yaml: line (\d+): (.*)`)

func (YAMLValidator) ValidateSyntax(b []byte) (bool, error) {
	var output any
	err := yaml.Unmarshal(b, &output)
	if err != nil {
		if m := yamlLineRe.FindStringSubmatch(err.Error()); m != nil {
			if line, convErr := strconv.Atoi(m[1]); convErr == nil {
				return false, &ValidationError{Err: errors.New(m[2]), Line: line}
			}
		}
		return false, err
	}

	return true, nil
}

func (YAMLValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
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

	posMap := buildYAMLPositionMap(b)
	return JSONSchemaValidateWithPositions(resolveSchemaURL(schemaURL, filePath), docJSON, posMap)
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

// buildYAMLPositionMap parses YAML into a Node tree and builds a map from
// gojsonschema context paths (e.g. "(root).server.port") to source positions.
func buildYAMLPositionMap(b []byte) map[string]SourcePosition {
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		return nil
	}
	positions := make(map[string]SourcePosition)
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		walkYAMLNode(root.Content[0], "(root)", positions)
	}
	return positions
}

func walkYAMLNode(node *yaml.Node, path string, positions map[string]SourcePosition) {
	switch node.Kind {
	case yaml.MappingNode:
		positions[path] = SourcePosition{Line: node.Line, Column: node.Column}
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			childPath := path + "." + keyNode.Value
			positions[childPath] = SourcePosition{Line: keyNode.Line, Column: keyNode.Column}
			walkYAMLNode(valNode, childPath, positions)
		}
	case yaml.SequenceNode:
		positions[path] = SourcePosition{Line: node.Line, Column: node.Column}
		for i, child := range node.Content {
			childPath := fmt.Sprintf("%s.%d", path, i)
			positions[childPath] = SourcePosition{Line: child.Line, Column: child.Column}
			walkYAMLNode(child, childPath, positions)
		}
	default:
	}
}
