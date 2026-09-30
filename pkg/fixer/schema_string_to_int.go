package fixer

import (
	"bytes"
	"fmt"
	"strconv"
)

// JSONStringToInt detects JSON string values that should be integers according
// to the provided JSON Schema, and produces fixes to coerce them.
type JSONStringToInt struct{}

var _ Rule = JSONStringToInt{}

// ID returns the rule identifier.
func (JSONStringToInt) ID() string { return "schema-string-to-int" }

// Detect finds string values at paths where the schema expects an integer,
// and returns fixes to replace the quoted string with an unquoted integer.
func (JSONStringToInt) Detect(src []byte, schema []byte, format string) []Fix {
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
		if !ok || expectedType != "integer" {
			continue
		}

		// Value must be a quoted string.
		if len(loc.raw) < 2 || loc.raw[0] != '"' || loc.raw[len(loc.raw)-1] != '"' {
			continue
		}

		// Extract string content (between quotes).
		content := string(loc.raw[1 : len(loc.raw)-1])

		// Validate it's a valid integer.
		if _, err := strconv.Atoi(content); err != nil {
			continue
		}

		line := 1 + bytes.Count(src[:loc.start], []byte("\n"))
		fixes = append(fixes, Fix{
			RuleID:      "schema-string-to-int",
			Message:     fmt.Sprintf("convert string %q to integer %s", content, content),
			Category:    FixSchema,
			Safety:      Safe,
			Line:        line,
			Start:       loc.start,
			End:         loc.end,
			Replacement: []byte(content),
		})
	}

	return fixes
}
