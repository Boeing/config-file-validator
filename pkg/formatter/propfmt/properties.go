// Package propfmt provides a Formatter for Java .properties files.
//
// The formatter uses a custom line-oriented approach rather than the
// magiconair/properties library's serializer, because the library does not
// correctly escape comment characters (! and #) in key position on output.
//
// The library is used only for validation (confirming the file parses).
// Formatting is done by walking source lines and normalizing spacing
// around the key-value separator.
//
// Comments (both # and !) are preserved verbatim.
package propfmt

import (
	"bytes"
	"slices"
	"strings"

	"github.com/magiconair/properties"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats .properties files.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for properties files.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  0,
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
		SortKeys:     false,
	}
}

// line represents a parsed line in a properties file.
type line struct {
	kind      lineKind
	raw       string // original content for comments/blanks/multiline kv
	key       string // for key-value lines (with escapes preserved)
	value     string // everything after separator (preserves continuation)
	comment   string // attached comment block preceding this key
	multiline bool   // true if value spans multiple lines via continuation
}

type lineKind int

const (
	kindBlank   lineKind = iota
	kindComment          // starts with # or !
	kindKeyVal           // key=value, key:value, or key value
)

// Format returns the canonically formatted version of src.
// Returns an error if src cannot be parsed as a properties file.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate with the library — catches malformed escape sequences.
	if _, err := properties.Load(src, properties.UTF8); err != nil {
		return nil, err
	}

	lines := parse(src)

	if opts.SortKeys {
		lines = sortLines(lines)
	}

	out := render(lines)

	// Ensure correct trailing newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	// Re-validate: if our formatted output doesn't parse back correctly
	// or isn't idempotent (edge cases with escapes + continuation), return
	// the source unchanged. The input was valid — we just can't safely
	// normalize it.
	if _, err := properties.Load(out, properties.UTF8); err != nil {
		return src, nil
	}

	// Idempotency check: re-format and compare.
	lines2 := parse(out)
	if opts.SortKeys {
		lines2 = sortLines(lines2)
	}
	out2 := render(lines2)
	out2 = bytes.TrimRight(out2, "\r\n")
	if opts.FinalNewline {
		out2 = append(out2, '\n')
	}
	out2 = formatter.NormalizeLineEndings(out2, opts.LineEnding)
	if !bytes.Equal(out, out2) {
		return src, nil
	}

	return out, nil
}

// parse splits src into classified lines.
func parse(src []byte) []line {
	rawLines := strings.Split(string(src), "\n")
	// Remove trailing empty string from split if src ends with \n.
	if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	var result []line
	var pendingComments []string
	var continuation bool

	for _, r := range rawLines {
		// Strip \r from CRLF.
		r = strings.TrimRight(r, "\r")

		// If previous line ended with \, this is a continuation.
		if continuation {
			// Append to previous key-value line's raw.
			if len(result) > 0 {
				prev := &result[len(result)-1]
				prev.raw += "\n" + r
				prev.multiline = true
			}
			continuation = endsWithContinuation(r)
			continue
		}

		trimmed := strings.TrimSpace(r)

		switch {
		case trimmed == "":
			// Blank line — flush pending comments.
			for _, c := range pendingComments {
				result = append(result, line{kind: kindComment, raw: c})
			}
			pendingComments = nil
			result = append(result, line{kind: kindBlank})

		case trimmed[0] == '#' || trimmed[0] == '!':
			pendingComments = append(pendingComments, r)

		default:
			// Key-value line.
			kv := parseKeyValue(r)
			kv.raw = r
			kv.comment = strings.Join(pendingComments, "\n")
			pendingComments = nil
			continuation = endsWithContinuation(r)
			result = append(result, kv)
		}
	}

	// Flush trailing comments.
	for _, c := range pendingComments {
		result = append(result, line{kind: kindComment, raw: c})
	}

	return result
}

// parseKeyValue parses a key-value line, preserving the key's escape sequences.
func parseKeyValue(s string) line {
	trimmed := strings.TrimLeft(s, " \t")

	// Find the separator: first unescaped =, :, or whitespace.
	keyEnd, sepEnd := findSeparator(trimmed)

	key := trimmed[:keyEnd]
	value := ""
	if sepEnd < len(trimmed) {
		value = strings.TrimLeft(trimmed[sepEnd:], " \t")
	}

	// If the key ends with a backslash, normalization is unsafe because
	// the backslash escapes the following character (space, =, :). Adding
	// or removing whitespace would change what's escaped. Preserve as raw.
	unsafeToNormalize := len(key) > 0 && key[len(key)-1] == '\\'

	return line{kind: kindKeyVal, key: key, value: value, raw: trimmed, multiline: unsafeToNormalize}
}

// findSeparator finds the end of the key and the end of the separator
// in a properties key-value line. Returns (keyEnd, valueStart).
func findSeparator(s string) (keyEnd int, valueStart int) {
	i := 0
	for i < len(s) {
		if s[i] == '\\' {
			i += 2 // skip escaped character
			continue
		}
		if s[i] == '=' || s[i] == ':' {
			// Key ends here, value starts after separator + optional whitespace.
			return i, i + 1
		}
		if s[i] == ' ' || s[i] == '\t' {
			// Whitespace separator — key ends, find where value starts.
			keyEnd := i
			for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			// Check if there's an = or : after the whitespace.
			if i < len(s) && (s[i] == '=' || s[i] == ':') {
				return keyEnd, i + 1
			}
			return keyEnd, i
		}
		i++
	}
	// No separator found — entire line is the key with empty value.
	return len(s), len(s)
}

// endsWithContinuation checks if a line ends with an odd number of backslashes
// (indicating line continuation in properties files).
func endsWithContinuation(s string) bool {
	count := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

// sortLines sorts key-value lines alphabetically by key while preserving
// their attached comment blocks.
func sortLines(lines []line) []line {
	var kvLines []line
	var header []line
	headerDone := false

	for _, l := range lines {
		if !headerDone && l.kind != kindKeyVal {
			header = append(header, l)
		} else {
			headerDone = true
			if l.kind == kindKeyVal {
				kvLines = append(kvLines, l)
			}
		}
	}

	slices.SortStableFunc(kvLines, func(a, b line) int {
		if a.key < b.key {
			return -1
		}
		if a.key > b.key {
			return 1
		}
		return 0
	})

	return append(header, kvLines...)
}

// render produces the formatted output from parsed lines.
func render(lines []line) []byte {
	var buf bytes.Buffer
	for _, l := range lines {
		switch l.kind {
		case kindBlank:
			buf.WriteByte('\n')
		case kindComment:
			buf.WriteString(l.raw)
			buf.WriteByte('\n')
		case kindKeyVal:
			if l.comment != "" {
				buf.WriteString(l.comment)
				buf.WriteByte('\n')
			}
			if l.multiline {
				// Multiline values (continuation) — preserve verbatim.
				// We cannot safely normalize these without understanding
				// the escape semantics deeply.
				buf.WriteString(l.raw)
				buf.WriteByte('\n')
			} else {
				buf.WriteString(l.key)
				buf.WriteString(" = ")
				buf.WriteString(l.value)
				buf.WriteByte('\n')
			}
		default:
			// unknown line kind
		}
	}
	return buf.Bytes()
}
