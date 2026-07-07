// Package inifmt provides a Formatter for INI files.
//
// The formatter uses a line-oriented approach: gopkg.in/ini.v1 validates
// syntax, then source lines are walked directly to normalize formatting.
// This avoids the library's Write methods which have round-trip issues
// with keys containing special characters (backticks, quotes).
//
// Comments (both # and ;) are preserved verbatim.
package inifmt

import (
	"bytes"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats INI files.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for INI files.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  0,
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src cannot be parsed as an INI file.
//
// Line continuation (trailing backslash joining next line) is disabled.
// Backslashes in values are treated as literal characters.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate with library.
	loadOpts := ini.LoadOptions{
		PreserveSurroundedQuote: true,
		IgnoreInlineComment:     true,
		IgnoreContinuation:      true,
	}
	if _, err := ini.LoadSources(loadOpts, src); err != nil {
		return nil, err
	}

	// Line-oriented formatting.
	indent := buildIndent(opts)
	lines := splitLines(src)
	inSection := false

	var buf bytes.Buffer
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			// Blank line — preserve.
			buf.WriteByte('\n')

		case trimmed[0] == '#' || trimmed[0] == ';':
			// Comment — preserve with indent if in section.
			if inSection && indent != "" {
				buf.WriteString(indent)
			}
			buf.WriteString(trimmed)
			buf.WriteByte('\n')

		case trimmed[0] == '[':
			// Section header — no indent.
			inSection = true
			buf.WriteString(trimmed)
			buf.WriteByte('\n')

		default:
			// Key-value line — normalize spacing around separator.
			normalized := normalizeKeyValue(trimmed)
			if inSection && indent != "" {
				buf.WriteString(indent)
			}
			buf.WriteString(normalized)
			buf.WriteByte('\n')
		}
	}

	out := buf.Bytes()

	// Ensure correct trailing newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	// Re-validate: if our formatted output doesn't parse back correctly
	// (edge cases with quoted values + trailing newline interaction),
	// return the source unchanged. This is safe — the input was valid,
	// we just can't safely normalize it.
	if _, err := ini.LoadSources(loadOpts, out); err != nil {
		return src, nil
	}

	return out, nil
}

// splitLines splits src into lines, handling both LF and CRLF.
func splitLines(src []byte) []string {
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	// Remove trailing empty element from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// normalizeKeyValue normalizes spacing around the separator (= or :) in
// an INI key-value line. Preserves the key and value verbatim — no
// quoting or escape transformation.
//
// Values starting with " or ' are left as-is because the ini.v1 parser
// interprets them as quoted strings, and adding/removing whitespace
// around the separator can change how the parser delimits the quote.
func normalizeKeyValue(s string) string {
	sepIdx := findSeparator(s)
	if sepIdx < 0 {
		// No separator — could be a boolean key (my.cnf style). Preserve as-is.
		return s
	}

	key := strings.TrimRight(s[:sepIdx], " \t")
	sep := s[sepIdx] // '=' or ':'
	value := strings.TrimLeft(s[sepIdx+1:], " \t")

	// If value starts with a quote character, preserve the line as-is.
	// The ini parser's quoted-value handling is sensitive to whitespace
	// positioning and we cannot safely normalize without risking a
	// re-parse failure.
	if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
		return s
	}

	return key + " " + string(sep) + " " + value
}

// findSeparator finds the index of the first unquoted = or : separator.
// Returns -1 if not found.
func findSeparator(s string) int {
	i := 0
	for i < len(s) {
		switch s[i] {
		case '\\':
			i += 2 // skip escaped character
		case '=', ':':
			return i
		default:
			i++
		}
	}
	return -1
}

// buildIndent constructs the indent string from options.
func buildIndent(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	if opts.IndentWidth <= 0 {
		return ""
	}
	return strings.Repeat(" ", opts.IndentWidth)
}
