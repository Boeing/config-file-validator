package validator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// YAMLValidator validates YAML files.
// Uses goccy/go-yaml which rejects duplicate keys by default and validates
// all documents in multi-doc files.
type YAMLValidator struct{}

var _ Validator = YAMLValidator{}

func (YAMLValidator) ValidateSyntax(b []byte) (bool, error) {
	// Decode all documents. goccy's Decoder checks duplicates and references.
	dec := goyaml.NewDecoder(bytes.NewReader(b))
	for {
		var output any
		err := dec.Decode(&output)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return false, parseValidationError(err)
		}
	}
	return true, nil
}

func (YAMLValidator) MarshalToJSON(b []byte) ([]byte, error) {
	var doc any
	if err := goyaml.Unmarshal(b, &doc); err != nil {
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
	if err := goyaml.Unmarshal(b, &doc); err != nil {
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

// buildYAMLPositionMap parses YAML into an AST and builds a map from
// gojsonschema context paths (e.g. "(root).server.port") to source positions.
func buildYAMLPositionMap(b []byte) map[string]SourcePosition {
	file, err := parser.ParseBytes(b, parser.ParseComments)
	if err != nil || len(file.Docs) == 0 {
		return nil
	}
	positions := make(map[string]SourcePosition)
	walkYAMLNode(file.Docs[0].Body, "(root)", positions)
	return positions
}

func walkYAMLNode(node ast.Node, path string, positions map[string]SourcePosition) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.MappingNode:
		tk := n.GetToken()
		if tk != nil {
			positions[path] = SourcePosition{Line: tk.Position.Line, Column: tk.Position.Column}
		}
		for _, mv := range n.Values {
			keyTk := mv.Key.GetToken()
			childPath := path + "." + keyTk.Value
			positions[childPath] = SourcePosition{Line: keyTk.Position.Line, Column: keyTk.Position.Column}
			walkYAMLNode(mv.Value, childPath, positions)
		}
	case *ast.SequenceNode:
		tk := n.GetToken()
		if tk != nil {
			positions[path] = SourcePosition{Line: tk.Position.Line, Column: tk.Position.Column}
		}
		for i, item := range n.Values {
			childPath := fmt.Sprintf("%s.%d", path, i)
			itemTk := item.GetToken()
			if itemTk != nil {
				positions[childPath] = SourcePosition{Line: itemTk.Position.Line, Column: itemTk.Position.Column}
			}
			walkYAMLNode(item, childPath, positions)
		}
	case *ast.MappingValueNode:
		// When a sequence contains mapping values directly
		walkYAMLNode(n.Value, path, positions)
	default:
		// Scalar, anchor, alias, tag nodes — no children to walk for positions.
	}
}

// parseValidationError extracts line/column from goccy's error format.
// goccy errors look like: "[3:5] error message\n   1 | ...\n>  3 | ...\n"
func parseValidationError(err error) error {
	msg := err.Error()
	// Try to extract [line:col] prefix
	if len(msg) > 0 && msg[0] == '[' {
		end := strings.Index(msg, "]")
		if end > 0 {
			coords := msg[1:end]
			parts := strings.SplitN(coords, ":", 2)
			if len(parts) == 2 {
				var line, col int
				if _, err := fmt.Sscanf(parts[0], "%d", &line); err == nil {
					_, _ = fmt.Sscanf(parts[1], "%d", &col)
					// Extract just the error message (after "] ")
					errMsg := msg[end+2:]
					// Trim trailing context lines
					if nlIdx := strings.Index(errMsg, "\n"); nlIdx > 0 {
						errMsg = errMsg[:nlIdx]
					}
					return &ValidationError{
						Err:    fmt.Errorf("%s", errMsg),
						Line:   line,
						Column: col,
					}
				}
			}
		}
	}
	return err
}
