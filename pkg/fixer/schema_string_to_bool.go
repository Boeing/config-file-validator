package fixer

import (
	"bytes"
	"fmt"
	"strings"
)

// JSONStringToBool detects JSON string values that should be booleans according
// to the provided JSON Schema, and produces fixes to coerce them.
type JSONStringToBool struct{}

var _ Rule = JSONStringToBool{}

// ID returns the rule identifier.
func (JSONStringToBool) ID() string { return "schema-string-to-bool" }

// Detect finds string values at paths where the schema expects a boolean,
// and returns fixes to replace the quoted string with an unquoted boolean.
func (JSONStringToBool) Detect(src []byte, schema []byte, format string) []Fix {
	if format != "json" || schema == nil {
		return nil
	}

	typeMap := schemaTypeMap(schema)
	if len(typeMap) == 0 {
		return nil
	}

	locations := jsonValueLocations(src)
	var fixes []Fix

	for _, loc := range locations {
		expectedType, ok := typeMap[loc.path]
		if !ok || expectedType != "boolean" {
			continue
		}

		// Value must be a quoted string.
		if len(loc.raw) < 2 || loc.raw[0] != '"' || loc.raw[len(loc.raw)-1] != '"' {
			continue
		}

		// Extract string content (between quotes).
		content := string(loc.raw[1 : len(loc.raw)-1])

		// Check if it's a boolean string (case-insensitive).
		var replacement string
		switch strings.ToLower(content) {
		case "true":
			replacement = "true"
		case "false":
			replacement = "false"
		default:
			continue
		}

		line := 1 + bytes.Count(src[:loc.start], []byte("\n"))
		fixes = append(fixes, Fix{
			RuleID:      "schema-string-to-bool",
			Message:     fmt.Sprintf("convert string %q to boolean %s", content, replacement),
			Category:    FixSchema,
			Safety:      Safe,
			Line:        line,
			Start:       loc.start,
			End:         loc.end,
			Replacement: []byte(replacement),
		})
	}

	return fixes
}
