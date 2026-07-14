// Package yamlfmt provides a Formatter for YAML files.
//
// The formatter uses github.com/goccy/go-yaml's token-preserving AST to parse,
// modify, and re-serialize YAML content. Comments, document markers, anchors,
// aliases, and quoting styles are preserved exactly by the token-based printer.
//
// Formatting operations (indent normalization, sort-keys, quote-style) modify
// the AST tokens in-place, then serialize via file.String().
//
// Defaults:
//   - 2-space indentation
//   - preserve existing quote style
//   - preserve key order
//   - trailing newline
//   - preserve document start (---) and end (...) markers
package yamlfmt

import (
	"bytes"
	"errors"
	"slices"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

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

	// Reject null bytes (YAML spec forbids them; goccy doesn't catch this).
	if bytes.ContainsRune(src, 0) {
		return nil, errors.New("yaml: source contains null byte")
	}

	resolved := resolveOptions(opts)

	// Parse into AST. This validates syntax, duplicate keys, and structure.
	file, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil, errors.New("yaml: " + err.Error())
	}

	// Validate semantics (undefined anchors, type errors) via Unmarshal.
	// The parser catches syntax and duplicate keys; Unmarshal catches references.
	var semanticCheck any
	if err := goyaml.Unmarshal(src, &semanticCheck); err != nil {
		return nil, errors.New("yaml: " + err.Error())
	}

	// Only format documents with a mapping or sequence at the root.
	// Bare scalars and other exotic root types are valid YAML but not
	// config files — fail fast with a clear message.
	if !hasFormattableRoot(file) {
		return nil, errors.New("yaml: cannot format (document root is not a mapping or sequence)")
	}

	// Apply indent normalization.
	reindent(file, resolved.IndentWidth)

	// Apply sort-keys and quote-style via AST walk.
	for _, doc := range file.Docs {
		normalizeNode(doc.Body, resolved)
	}

	// Serialize from the modified AST.
	result := []byte(file.String())

	// Verify the output is valid and stable. If the AST serializer produced
	// output that can't be re-parsed or that isn't stable on re-serialization,
	// the input contains YAML constructs we can't safely reformat. Fail fast
	// with a clear error rather than producing potentially incorrect output.
	reparsed, err := parser.ParseBytes(result, parser.ParseComments)
	if err != nil {
		return nil, errors.New("yaml: formatter produced invalid output (unsupported YAML construct)")
	}
	if !hasFormattableRoot(reparsed) {
		return nil, errors.New("yaml: formatter produced unstable output (unsupported YAML construct)")
	}
	// Also verify semantics of the output (catches merge key issues etc).
	var outputCheck any
	if err := goyaml.Unmarshal(result, &outputCheck); err != nil {
		return nil, errors.New("yaml: formatter produced invalid output (unsupported YAML construct)")
	}
	reparsedStr := reparsed.String()
	resultStr := string(bytes.TrimRight(result, "\n"))
	if reparsedStr != resultStr && reparsedStr != resultStr+"\n" {
		return nil, errors.New("yaml: formatter produced unstable output (unsupported YAML construct)")
	}

	// Ensure exactly one trailing newline.
	if resolved.FinalNewline {
		result = bytes.TrimRight(result, "\n")
		result = append(result, '\n')
	} else if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return formatter.NormalizeLineEndings(result, resolved.LineEnding), nil
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

// =============================================================================
// Indent normalization
// =============================================================================

// reindent walks the AST using structural depth tracking and uses AddColumn
// to adjust node positions. This correctly handles sequences, nested mappings,
// and all combinations.
func reindent(file *ast.File, targetIndent int) {
	for _, doc := range file.Docs {
		reindentByDepth(doc.Body, 0, targetIndent)
	}
}

func reindentByDepth(node ast.Node, depth, targetIndent int) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.MappingNode:
		// Flow-style mappings ({key: val}) are inline — don't reindent.
		if n.IsFlowStyle {
			return
		}
		for _, mv := range n.Values {
			currentCol := mv.Key.GetToken().Position.Column
			targetCol := depth*targetIndent + 1
			delta := targetCol - currentCol
			if delta != 0 && targetCol >= 1 {
				mv.AddColumn(delta)
			}
			// Recurse into value at depth+1
			reindentByDepth(mv.Value, depth+1, targetIndent)
		}
	case *ast.SequenceNode:
		// Flow-style sequences ([a, b]) are inline — don't reindent.
		if n.IsFlowStyle {
			return
		}
		// Adjust the sequence start position
		currentCol := n.Start.Position.Column
		targetCol := depth*targetIndent + 1
		delta := targetCol - currentCol
		if delta != 0 && targetCol >= 1 {
			n.Start.AddColumn(delta)
			if n.End != nil {
				n.End.AddColumn(delta)
			}
		}
		for _, item := range n.Values {
			// Items inside a sequence: adjust relative to sequence position
			itemTk := item.GetToken()
			if itemTk != nil {
				itemCurrentCol := itemTk.Position.Column
				itemTargetCol := targetCol + 2 // after "- "
				itemDelta := itemTargetCol - itemCurrentCol
				if itemDelta != 0 && itemTargetCol >= 1 {
					item.AddColumn(itemDelta)
				}
			}
			// Recurse: mappings inside sequence items are at depth+1
			reindentByDepth(item, depth+1, targetIndent)
		}
	case *ast.MappingValueNode:
		reindentByDepth(n.Value, depth, targetIndent)
	case *ast.AnchorNode:
		reindentByDepth(n.Value, depth, targetIndent)
	case *ast.TagNode:
		reindentByDepth(n.Value, depth, targetIndent)
	default:
		// Scalars, aliases — no children.
	}
}

// =============================================================================
// AST normalization (sort-keys, quote-style)
// =============================================================================

// normalizeNode walks an AST node and applies style normalisations.
func normalizeNode(node ast.Node, opts formatter.Options) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.MappingNode:
		if opts.SortKeys && len(n.Values) >= 2 {
			sortMappingKeys(n)
		}
		for _, mv := range n.Values {
			// Apply quote-style to values only (not keys).
			if opts.QuoteStyle != formatter.QuotePreserve {
				applyQuoteStyleToValue(mv.Value, opts.QuoteStyle)
			}
			normalizeNode(mv.Value, opts)
		}
	case *ast.MappingValueNode:
		normalizeNode(n.Value, opts)
	case *ast.SequenceNode:
		for _, item := range n.Values {
			if opts.QuoteStyle != formatter.QuotePreserve {
				applyQuoteStyleToValue(item, opts.QuoteStyle)
			}
			normalizeNode(item, opts)
		}
	case *ast.TagNode:
		normalizeNode(n.Value, opts)
	case *ast.AnchorNode:
		normalizeNode(n.Value, opts)
	default:
		// Scalar nodes, aliases, etc. — no children to recurse into.
	}
}

// sortMappingKeys sorts a MappingNode's values alphabetically by key.
func sortMappingKeys(node *ast.MappingNode) {
	slices.SortStableFunc(node.Values, func(a, b *ast.MappingValueNode) int {
		aKey := a.Key.GetToken().Value
		bKey := b.Key.GetToken().Value
		return strings.Compare(aKey, bKey)
	})
}

// applyQuoteStyleToValue changes the quoting of a scalar value node.
// Only modifies string scalars that are already quoted.
func applyQuoteStyleToValue(node ast.Node, style formatter.QuoteStyle) {
	if node == nil {
		return
	}
	str, ok := node.(*ast.StringNode)
	if !ok {
		return
	}
	tk := str.GetToken()
	if tk == nil {
		return
	}
	// Only modify already-quoted scalars.
	if tk.Type != token.SingleQuoteType && tk.Type != token.DoubleQuoteType {
		return
	}
	switch style {
	case formatter.QuoteDouble:
		if tk.Type == token.SingleQuoteType {
			tk.Type = token.DoubleQuoteType
			// Rewrite Origin: 'value' → "value"
			tk.Origin = strings.Replace(tk.Origin, "'"+tk.Value+"'", "\""+tk.Value+"\"", 1)
		}
	case formatter.QuoteSingle:
		if tk.Type == token.DoubleQuoteType {
			tk.Type = token.SingleQuoteType
			// Rewrite Origin: "value" → 'value'
			tk.Origin = strings.Replace(tk.Origin, "\""+tk.Value+"\"", "'"+tk.Value+"'", 1)
		}
	default:
		// QuotePreserve — no change.
	}
}

// hasFormattableRoot returns true if every document in the file has a mapping
// or sequence as its body. Bare scalars and other exotic root types are valid
// YAML but aren't config files — they're returned unchanged by Format.
func hasFormattableRoot(file *ast.File) bool {
	hasContent := false
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		hasContent = true
		switch doc.Body.(type) {
		case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
			// formattable
		default:
			return false
		}
	}
	return hasContent
}
