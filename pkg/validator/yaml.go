package validator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLValidator validates YAML files.
// Uses gopkg.in/yaml.v3 which rejects duplicate keys by default and validates
// all documents in multi-doc files via the Decoder loop.
type YAMLValidator struct{}

var _ Validator = YAMLValidator{}

// yamlLineRe extracts line number from yaml.v3 error messages.
// Formats: "yaml: line 3: ..." or "  line 2: mapping key ..."
var yamlLineRe = regexp.MustCompile(`(?:yaml: )?line (\d+): (.*)`)

// ValidateSyntax validates YAML syntax across all documents in the file.
func (YAMLValidator) ValidateSyntax(b []byte) (bool, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var output any
		err := dec.Decode(&output)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return false, parseYAMLError(err)
		}
	}
	return true, nil
}

// MarshalToJSON converts YAML to JSON for schema validation.
func (YAMLValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

// ValidateSchema validates YAML against a JSON Schema referenced via comment.
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
		// Scalar and other nodes — no children to walk for positions.
	}
}

// parseYAMLError extracts line number from yaml.v3 error messages and wraps
// in ValidationError for structured error reporting.
func parseYAMLError(err error) error {
	msg := err.Error()
	// Handle multi-error format: "yaml: unmarshal errors:\n  line 2: ..."
	if strings.HasPrefix(msg, "yaml: unmarshal errors:") {
		lines := strings.Split(msg, "\n")
		if len(lines) > 1 {
			msg = strings.TrimSpace(lines[1])
		}
	}
	if m := yamlLineRe.FindStringSubmatch(msg); m != nil {
		if line, convErr := strconv.Atoi(m[1]); convErr == nil {
			return &ValidationError{Err: errors.New(m[2]), Line: line}
		}
	}
	return err
}
