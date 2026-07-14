// Package yamlfmt provides a Formatter for YAML files.
//
// The formatter uses a CST-based pipeline:
//   - gopkg.in/yaml.v3 validates syntax (rejects invalid YAML)
//   - Custom tokenizer produces a lossless token stream
//   - Printer normalizes indentation, optionally sorts keys, applies quote style
//
// Comments, document markers, anchors, aliases, block scalars, and flow
// collections are preserved verbatim through the format cycle.
//
// Defaults:
//   - 2-space indentation
//   - preserve existing quote style
//   - preserve key order
//   - trailing newline
package yamlfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats YAML files to canonical style.
// It is stateless and safe for concurrent use.
type Formatter struct{}

// compile-time check.
var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for YAML.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  2,
		FinalNewline: true,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src cannot be parsed as valid YAML.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, &formatter.ErrSkipped{Reason: "empty document"}
	}

	// YAML spec requires spaces for indentation — tabs are not permitted.
	if opts.IndentStyle == formatter.IndentTabs {
		return nil, errors.New("yaml: tab indentation is not supported (YAML spec requires spaces)")
	}

	// Reject null bytes (YAML spec forbids them).
	if bytes.ContainsRune(src, 0) {
		return nil, errors.New("yaml: source contains null byte")
	}

	// Validate with yaml.v3. We validate the form WITH a trailing newline
	// (since our output always has one) to avoid parser inconsistencies where
	// yaml.v3 accepts input without newline but rejects it with one.
	toValidate := src
	if len(src) > 0 && src[len(src)-1] != '\n' {
		toValidate = append(bytes.Clone(src), '\n')
	}
	dec := yaml.NewDecoder(bytes.NewReader(toValidate))
	var firstDoc any
	for {
		var doc any
		err := dec.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("yaml: %w", err)
		}
		if firstDoc == nil {
			firstDoc = doc
		}
	}

	// Only format documents whose root is a mapping or sequence.
	// Bare scalars are valid YAML but not config files.
	// Nil roots (empty documents, binary garbage parsed as nil) have nothing to format.
	switch firstDoc.(type) {
	case map[string]any, []any:
		// formattable
	default:
		return nil, errors.New("yaml: cannot format (document root is not a mapping or sequence)")
	}

	resolved := resolveOptions(opts)

	// Build structural line map from yaml.v3 Node tree.
	// This tells us which lines contain structural elements (keys, sequence items)
	// vs continuation lines (multi-line scalar values).
	structuralLines := buildStructuralLineMap(toValidate)

	// CST-based pipeline: tokenize → format → print.
	tokens := tokenize(src)
	out := printFormatted(tokens, resolved, structuralLines)

	return out, nil
}

// resolveOptions fills zero-value Options with YAML defaults.
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

// buildStructuralLineMap parses the YAML into a Node tree and returns
// a set of line numbers that contain structural elements (mapping keys,
// sequence items). Lines NOT in this set are continuation lines (multi-line
// scalar values) and should not be independently reindented.
func buildStructuralLineMap(src []byte) map[int]bool {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		// If parse fails, treat all lines as structural (safe default).
		return nil
	}
	lines := make(map[int]bool)
	collectStructuralLines(&root, lines)
	return lines
}

func collectStructuralLines(n *yaml.Node, lines map[int]bool) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			collectStructuralLines(c, lines)
		}
	case yaml.MappingNode:
		// The mapping node itself is on a structural line.
		lines[n.Line] = true
		// Every key in a mapping starts a structural line.
		for i := 0; i < len(n.Content); i += 2 {
			lines[n.Content[i].Line] = true
			// Recurse into the value to find nested structural elements.
			if i+1 < len(n.Content) {
				collectStructuralLines(n.Content[i+1], lines)
			}
		}
	case yaml.SequenceNode:
		// The sequence node itself is on a structural line.
		lines[n.Line] = true
		// Every item in a sequence starts a structural line.
		for _, item := range n.Content {
			lines[item.Line] = true
			collectStructuralLines(item, lines)
		}
	}
}
