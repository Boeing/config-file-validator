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
	"strings"

	"github.com/lestrrat-go/helium"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// bom is the UTF-8 Byte Order Mark.
var bom = []byte{0xef, 0xbb, 0xbf}

// Formatter formats XML files using DOM-based serialization.
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
// Returns *formatter.ErrSkipped if the document contains mixed content
// that cannot be safely reformatted.
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

	ctx := context.Background()
	doc, err := helium.NewParser().StripBlanks(true).Parse(ctx, input)
	if err != nil {
		return nil, err
	}

	// Detect mixed content before formatting.
	if hasMixedContent(doc) {
		return nil, &formatter.ErrSkipped{Reason: "contains mixed content"}
	}

	indent := buildIndent(opts)

	// Preserve XML declaration if the source has one.
	hasDecl := bytes.Contains(input, []byte("<?xml"))

	w := helium.NewWriter().
		Format(true).
		IndentString(indent).
		XMLDeclaration(hasDecl).
		SelfCloseEmptyElements(true)

	var buf bytes.Buffer
	if err := w.WriteTo(&buf, doc); err != nil {
		return nil, err
	}

	out := buf.Bytes()

	// Restore BOM if present.
	if hasBOM {
		out = append(bom, out...)
	}

	// Ensure correct trailing newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	return out, nil
}

// hasMixedContent walks the DOM tree and returns true if any element
// has both non-whitespace text children and element children.
func hasMixedContent(node helium.Node) bool {
	return walkForMixedContent(node)
}

// walkForMixedContent recursively checks nodes for mixed content.
func walkForMixedContent(node helium.Node) bool {
	if node == nil {
		return false
	}

	// Check if this node is an element with mixed content.
	if node.Type() == helium.ElementNode {
		hasElementChild := false
		hasTextContent := false

		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			switch child.Type() {
			case helium.ElementNode:
				hasElementChild = true
			case helium.TextNode:
				if strings.TrimSpace(string(child.Content())) != "" {
					hasTextContent = true
				}
			default:
				// comments, PI, etc. — not relevant for mixed content detection
			}
		}

		if hasElementChild && hasTextContent {
			return true
		}
	}

	// Recurse into children.
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if walkForMixedContent(child) {
			return true
		}
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
		width = 2
	}
	return strings.Repeat(" ", width)
}
