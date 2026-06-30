// Package jsonfmt provides a Formatter for JSON files.
//
// The formatter uses github.com/tidwall/pretty for canonical output.
// JSON has no comments, so comment preservation is not applicable.
//
// Defaults:
//   - 2-space indentation
//   - sorted keys
//   - trailing newline
package jsonfmt

import (
	"encoding/json"
	"errors"

	"github.com/tidwall/pretty"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats JSON files to canonical style.
// It is stateless and safe for concurrent use.
type Formatter struct{}

// compile-time check that Formatter implements formatter.Formatter.
var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for JSON.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  2,
		FinalNewline: true,
		SortKeys:     true,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid JSON.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if !json.Valid(src) {
		return nil, errors.New("json: invalid JSON input")
	}

	resolved := resolveOptions(opts)

	prettyOpts := &pretty.Options{
		Width:    resolved.MaxLineWidth,
		Prefix:   "",
		Indent:   indentString(resolved),
		SortKeys: resolved.SortKeys,
	}

	result := pretty.PrettyOptions(src, prettyOpts)

	// pretty always appends a trailing newline. Strip it if FinalNewline is false.
	if !resolved.FinalNewline && len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return normalizeLineEndings(result, resolved.LineEnding), nil
}

// resolveOptions fills zero-value options with JSON defaults.
func resolveOptions(opts formatter.Options) formatter.Options {
	defaults := DefaultOptions()
	if opts.IndentStyle == formatter.IndentDefault {
		opts.IndentStyle = defaults.IndentStyle
	}
	if opts.IndentWidth == 0 {
		opts.IndentWidth = defaults.IndentWidth
	}
	return opts
}

func indentString(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		width = 2 //nolint:mnd // JSON default
	}
	indent := make([]byte, width)
	for i := range indent {
		indent[i] = ' '
	}
	return string(indent)
}

func normalizeLineEndings(data []byte, ending formatter.LineEnding) []byte {
	if ending != formatter.LineEndingCRLF {
		return data
	}
	// Replace bare \n with \r\n (skip already-CRLF sequences).
	result := make([]byte, 0, len(data)+len(data)/10)
	for i, b := range data {
		if b == '\n' && (i == 0 || data[i-1] != '\r') {
			result = append(result, '\r')
		}
		result = append(result, b)
	}
	return result
}
