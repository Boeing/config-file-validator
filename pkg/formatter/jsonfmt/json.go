// Package jsonfmt provides a Formatter for JSON files.
//
// The formatter uses github.com/tailscale/hujson for canonical output.
// JSON is treated as a strict subset of JSONC — parsed with hujson,
// formatted with the shared JSONC format engine, trailing commas removed.
//
// Defaults:
//   - 2-space indentation
//   - original key order preserved
//   - trailing newline
//   - arrays/objects collapsed to one line when they fit within 80 columns
package jsonfmt

import (
	"encoding/json"
	"errors"

	"github.com/tailscale/hujson"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
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
		SortKeys:     false,
		MaxLineWidth: 80,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid JSON.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if !json.Valid(src) {
		return nil, errors.New("json: invalid JSON input")
	}

	v, err := hujson.Parse(src)
	if err != nil {
		return nil, err
	}

	// JSON forbids trailing commas — force removal.
	opts.TrailingCommas = formatter.TrailingCommasNone

	// Apply defaults for unset options.
	resolved := resolveOptions(opts)

	return jsoncfmt.FormatValue(&v, resolved)
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
	if opts.MaxLineWidth == 0 {
		opts.MaxLineWidth = defaults.MaxLineWidth
	}
	return opts
}
