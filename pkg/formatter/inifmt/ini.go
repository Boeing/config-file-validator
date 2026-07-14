// Package inifmt provides a Formatter for INI files.
//
// The formatter uses a CST-based pipeline:
//   - gopkg.in/ini.v1 validates syntax
//   - Custom tokenizer produces a lossless token stream
//   - Parser builds a Section→Entry tree
//   - Printer normalizes formatting and optionally sorts keys
//
// Comments (both # and ;) are preserved through the format cycle.
// Quoted values are preserved verbatim — no interpretation or transformation.
package inifmt

import (
	"fmt"

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
		SortKeys:     false,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src cannot be parsed as an INI file.
//
// Line continuation (trailing backslash joining next line) is disabled.
// Backslashes in values are treated as literal characters.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate with library — catches structural errors the tokenizer won't.
	loadOpts := ini.LoadOptions{
		PreserveSurroundedQuote: true,
		IgnoreInlineComment:     true,
		IgnoreContinuation:      true,
	}
	if _, err := ini.LoadSources(loadOpts, src); err != nil {
		return nil, err
	}

	// Tokenize → Parse → Print (CST-based pipeline).
	tokens := tokenize(src)
	file := parse(tokens)
	out := printFormatted(file, opts)

	// Verify: the formatted output must still parse. If it doesn't, the
	// formatter changed something that breaks ini.v1's parser (e.g., adding
	// a newline after a value with unbalanced quotes). Return an error
	// rather than silently returning the original (no bail-outs).
	if _, err := ini.LoadSources(loadOpts, out); err != nil {
		return nil, fmt.Errorf("formatted output is not valid INI: %w", err)
	}

	return out, nil
}
