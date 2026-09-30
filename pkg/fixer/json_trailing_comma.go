package fixer

import "bytes"

// JSONTrailingComma removes trailing commas before ] and } in JSON.
// This is a safe syntax fix — trailing commas are never valid in JSON.
type JSONTrailingComma struct{}

var _ Rule = JSONTrailingComma{}

// ID returns the rule identifier.
func (JSONTrailingComma) ID() string { return "json-trailing-comma" }

// Detect finds all trailing commas in JSON source.
func (JSONTrailingComma) Detect(src []byte, _ []byte, format string) []Fix {
	if format != "json" {
		return nil
	}

	var fixes []Fix
	i := 0
	for i < len(src) {
		switch src[i] {
		case '"':
			// Skip string literals.
			i = skipJSONString(src, i)
		case ',':
			// Check if this comma is followed only by whitespace and then ] or }.
			end := i + 1
			j := end
			for j < len(src) && isJSONWhitespace(src[j]) {
				j++
			}
			if j < len(src) && (src[j] == ']' || src[j] == '}') {
				line := 1 + bytes.Count(src[:i], []byte("\n"))
				fixes = append(fixes, Fix{
					RuleID:      "json-trailing-comma",
					Message:     "trailing comma before " + string(src[j]),
					Category:    FixSyntax,
					Safety:      Safe,
					Line:        line,
					Start:       i,
					End:         end,
					Replacement: nil, // delete the comma
				})
			}
			i++
		default:
			i++
		}
	}
	return fixes
}

// skipJSONString advances past a JSON string starting at src[start].
// Returns the index after the closing quote.
func skipJSONString(src []byte, start int) int {
	i := start + 1 // skip opening "
	for i < len(src) {
		if src[i] == '\\' {
			i += 2 // skip escape sequence
			continue
		}
		if src[i] == '"' {
			return i + 1
		}
		i++
	}
	return i
}

// isJSONWhitespace returns true for JSON whitespace characters.
func isJSONWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
