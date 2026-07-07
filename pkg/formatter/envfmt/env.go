// Package envfmt provides a Formatter for .env files.
//
// The formatter is a custom line-oriented implementation with no external
// dependencies. It normalizes whitespace around delimiters and ensures
// consistent formatting while preserving comments.
package envfmt

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats .env files.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for .env files.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  0,
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
		SortKeys:     false,
	}
}

// line represents a parsed line in an env file.
type line struct {
	kind    lineKind
	raw     string // original content (for comments/blanks)
	key     string // for key-value lines
	value   string // for key-value lines (preserves quotes)
	export  bool   // "export " prefix
	comment string // attached comment block (lines preceding this key)
}

type lineKind int

const (
	kindBlank   lineKind = iota
	kindComment          // starts with # or !
	kindKeyVal           // KEY=VALUE or export KEY=VALUE
)

// Format returns the canonically formatted version of src.
// Returns an error if src contains malformed lines (non-blank, non-comment
// lines without an = delimiter).
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	lines, err := parse(src)
	if err != nil {
		return nil, err
	}

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

	return out, nil
}

// parse splits src into classified lines.
func parse(src []byte) ([]line, error) {
	raw := strings.Split(string(src), "\n")
	// Remove trailing empty string from split if src ends with \n.
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	var result []line
	var pendingComments []string

	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		// Handle \r from CRLF input.
		trimmed = strings.TrimRight(trimmed, "\r")

		switch {
		case trimmed == "":
			// Blank lines flush pending comments as standalone.
			for _, c := range pendingComments {
				result = append(result, line{kind: kindComment, raw: c})
			}
			pendingComments = nil
			result = append(result, line{kind: kindBlank})

		case strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!"):
			pendingComments = append(pendingComments, trimmed)

		default:
			// Key-value line.
			kv, err := parseKeyValue(trimmed)
			if err != nil {
				return nil, err
			}
			kv.comment = strings.Join(pendingComments, "\n")
			pendingComments = nil
			result = append(result, kv)
		}
	}

	// Flush remaining comments.
	for _, c := range pendingComments {
		result = append(result, line{kind: kindComment, raw: c})
	}

	return result, nil
}

// parseKeyValue parses a KEY=VALUE or export KEY=VALUE line.
func parseKeyValue(s string) (line, error) {
	export := false
	work := s
	if strings.HasPrefix(work, "export ") {
		export = true
		work = strings.TrimPrefix(work, "export ")
	}

	idx := strings.IndexByte(work, '=')
	if idx < 0 {
		return line{}, errors.New("env: malformed line (no = delimiter): " + s)
	}

	key := strings.TrimSpace(work[:idx])
	value := work[idx+1:]

	return line{kind: kindKeyVal, key: key, value: value, export: export}, nil
}

// sortLines sorts key-value lines alphabetically while preserving
// their attached comment blocks.
func sortLines(lines []line) []line {
	// Separate key-value lines (with comments) from structural lines.
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
			// Skip standalone blanks between keys when sorting.
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
			if l.export {
				buf.WriteString("export ")
			}
			buf.WriteString(l.key)
			buf.WriteByte('=')
			buf.WriteString(l.value)
			buf.WriteByte('\n')
		default:
			// unknown line kind — should not happen
		}
	}
	return buf.Bytes()
}
