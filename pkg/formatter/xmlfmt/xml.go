// Package xmlfmt provides a Formatter for XML files.
//
// The formatter uses the helium library (already a dependency for XML
// validation) for DOM-based parsing and serialization. This provides
// correct indentation handling and comment preservation.
//
// Mixed content (elements with both text and child element siblings)
// cannot be safely indented without changing document semantics. When
// mixed content is detected, the formatter returns ErrSkipped so the
// CLI can notify the user.
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

// hasMixedContent walks the DOM tree and returns true if any element

