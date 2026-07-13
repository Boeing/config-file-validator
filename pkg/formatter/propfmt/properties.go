// Package propfmt provides a Formatter for Java .properties files.
//
// The formatter uses a CST-based pipeline:
//   - magiconair/properties validates syntax
//   - Custom tokenizer produces a lossless token stream
//   - Printer groups entries, normalizes separator spacing, and optionally sorts keys
//
// Comments (both # and !) are preserved through the format cycle.
// Continuation lines (trailing \) are preserved verbatim.
package propfmt

import (
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

// Format returns the canonically formatted version of src.
// Returns an error if src cannot be parsed as a properties file.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate with the library — catches malformed escape sequences.
	if _, err := properties.Load(src, properties.UTF8); err != nil {
		return nil, err
	}

	// Tokenize → Group → Print (CST-based pipeline).
	tokens := tokenize(src)
	out := printFormatted(tokens, opts)

	return out, nil
}
