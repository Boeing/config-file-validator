// Package tomlfmt provides a Formatter for TOML files.
//
// The formatter uses a line-oriented approach to preserve comments:
//   - pelletier/go-toml/v2 validates syntax (Unmarshal into any)
//   - Source lines are walked and classified (comment, blank, section, key-value)
//   - Spacing around "=" on single-line key-value pairs is normalized
//   - Comments, blank lines, and multiline values are preserved verbatim
//   - SortKeys is a no-op for TOML (would require deep scope analysis)
//
// This approach preserves all comments (standalone and inline) because
// it operates on source lines rather than reconstructing from an AST.
package tomlfmt

import (
	"bytes"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats TOML files with comment preservation.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for TOML.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  0, // TOML convention: no indentation
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
		SortKeys:     false,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid TOML.
//
// Comments (both standalone and inline) are preserved. Formatting
// normalizes spacing around "=" on single-line key-value pairs and
// applies indentation to table contents when configured.
//
// SortKeys is a no-op for TOML — sorting keys would require deep
// understanding of table scopes and would complicate comment association.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate syntax first.
	var discard any
	if err := toml.Unmarshal(src, &discard); err != nil {
		return nil, err
	}

	indent := buildIndent(opts)
	lines := splitLines(src)
	inTable := false
	inMultiline := false

	var buf bytes.Buffer
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track multiline values (triple-quoted strings, multiline arrays).
		if inMultiline {
			buf.WriteString(line)
			buf.WriteByte('\n')
			if isMultilineEnd(trimmed, line) {
				inMultiline = false
			}
			continue
		}

		switch {
		case trimmed == "":
			// Blank line — preserve.
			buf.WriteByte('\n')

		case strings.HasPrefix(trimmed, "#"):
			// Comment line — preserve with indent if in table.
			if inTable && indent != "" {
				buf.WriteString(indent)
			}
			buf.WriteString(trimmed)
			buf.WriteByte('\n')

		case strings.HasPrefix(trimmed, "["):
			// Section header — no indent.
			inTable = true
			buf.WriteString(trimmed)
			buf.WriteByte('\n')

		default:
			// Key-value line. Normalize spacing around "=".
			lineIndent := ""
			if inTable {
				lineIndent = indent
			}
			if isMultilineStart(line) {
				inMultiline = true
				normalized := normalizeKeyValue(line, lineIndent)
				buf.WriteString(normalized)
				buf.WriteByte('\n')
			} else {
				normalized := normalizeKeyValue(line, lineIndent)
				buf.WriteString(normalized)
				buf.WriteByte('\n')
			}
		}
	}

	out := buf.Bytes()

	// Ensure correct trailing newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	return out, nil
}

// splitLines splits src into lines, stripping the trailing \n from each.
// Handles both LF and CRLF input.
func splitLines(src []byte) []string {
	s := string(src)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	// Remove trailing empty element from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// normalizeKeyValue normalizes spacing around "=" in a key-value line.
// Preserves inline comments. Applies indent if inside a table.
func normalizeKeyValue(line, indent string) string {
	trimmed := strings.TrimSpace(line)

	// Find the first unquoted "=" that's part of the key-value separator.
	eqIdx := findEquals(trimmed)
	if eqIdx < 0 {
		// Not a simple key=value (could be dotted key with complex quoting).
		// Preserve as-is with indent.
		return indent + trimmed
	}

	key := strings.TrimSpace(trimmed[:eqIdx])
	rest := trimmed[eqIdx+1:]

	// Rest may contain inline comment. Preserve it.
	value := strings.TrimSpace(rest)

	return indent + key + " = " + value
}

// findEquals finds the index of the first "=" that acts as a key-value
// separator (not inside a quoted string).
func findEquals(s string) int {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '"':
			i = skipBasicString(s, i)
		case '\'':
			i = skipLiteralString(s, i)
		case '=':
			return i
		default:
			i++
		}
	}
	return -1
}

// skipBasicString advances past a basic (double-quoted) TOML string,
// handling escape sequences. Returns the index after the closing quote.
func skipBasicString(s string, start int) int {
	i := start + 1 // skip opening "
	for i < len(s) {
		if s[i] == '\\' {
			i += 2 // skip escape sequence
			continue
		}
		if s[i] == '"' {
			return i + 1 // past closing "
		}
		i++
	}
	return i
}

// skipLiteralString advances past a literal (single-quoted) TOML string.
// Returns the index after the closing quote.
func skipLiteralString(s string, start int) int {
	i := start + 1 // skip opening '
	for i < len(s) {
		if s[i] == '\'' {
			return i + 1 // past closing '
		}
		i++
	}
	return i
}

// isMultilineStart detects if a line starts a multiline value.
// Multiline values: triple-quoted strings (""" or ”'), arrays that
// don't close on the same line, inline tables that don't close.
func isMultilineStart(line string) bool {
	eqIdx := findEquals(strings.TrimSpace(line))
	if eqIdx < 0 {
		return false
	}
	value := strings.TrimSpace(strings.TrimSpace(line)[eqIdx+1:])

	// Triple-quoted strings.
	if strings.HasPrefix(value, `"""`) && !strings.HasSuffix(value, `"""`) {
		return true
	}
	if strings.HasPrefix(value, `'''`) && !strings.HasSuffix(value, `'''`) {
		return true
	}

	// Check for triple-quoted that also ends with triple-quote (single-line triple-quoted).
	if strings.HasPrefix(value, `"""`) && strings.Count(value, `"""`) >= 2 {
		return false
	}
	if strings.HasPrefix(value, `'''`) && strings.Count(value, `'''`) >= 2 {
		return false
	}

	// Array that doesn't close on the same line.
	if strings.HasPrefix(value, "[") && !strings.Contains(value, "]") {
		return true
	}

	// Inline table that doesn't close on the same line.
	if strings.HasPrefix(value, "{") && !strings.Contains(value, "}") {
		return true
	}

	return false
}

// isMultilineEnd detects if a line ends a multiline value.
func isMultilineEnd(trimmed, _ string) bool {
	// End of triple-quoted strings.
	if strings.HasSuffix(trimmed, `"""`) || strings.HasSuffix(trimmed, `'''`) {
		return true
	}
	// End of multiline array.
	if strings.HasSuffix(trimmed, "]") {
		return true
	}
	// End of multiline inline table.
	if strings.HasSuffix(trimmed, "}") {
		return true
	}
	return false
}

// buildIndent constructs the indent string from options.
func buildIndent(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		return "" // TOML convention: no indentation
	}
	return strings.Repeat(" ", width)
}
