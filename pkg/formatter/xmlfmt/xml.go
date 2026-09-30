// Package xmlfmt provides a Formatter for XML files.
//
// The formatter uses a custom CST-based pipeline: tokenize the raw bytes
// into a lossless token stream, annotate depth, detect mixed content,
// then rebuild with proper indentation. helium is used only for validation
// (parsing to confirm well-formedness), not for serialization.
//
// Mixed content (elements containing both text and child elements) is
// preserved verbatim — no formatting whitespace is inserted within such
// elements.
package xmlfmt

import (
	"bytes"
	"context"

	"github.com/lestrrat-go/helium"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// bom is the UTF-8 Byte Order Mark.
var bom = []byte{0xef, 0xbb, 0xbf}

// Formatter formats XML files using a CST-based pipeline.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for XML.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  2,
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid XML.
// Handles mixed content correctly by preserving it verbatim.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, &formatter.ErrSkipped{Reason: "empty XML document"}
	}

	// Strip BOM if present — restore after formatting.
	hasBOM := bytes.HasPrefix(src, bom)
	input := src
	if hasBOM {
		input = src[len(bom):]
	}

	// Validate with helium — rejects malformed XML.
	ctx := context.Background()
	if _, err := helium.NewParser().Parse(ctx, input); err != nil {
		return nil, err
	}

	// CST-based pipeline: tokenize → annotate → reindent → serialize.
	tokens := tokenize(input)
	out := printFormatted(tokens, opts, input)

	// Restore BOM if present.
	if hasBOM {
		out = append(bom, out...)
	}

	return out, nil
}
