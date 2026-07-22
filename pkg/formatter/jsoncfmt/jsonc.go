// Package jsoncfmt provides a Formatter for JSONC (JSON with Comments) files.
//
// The formatter uses tailscale/hujson's CST (concrete syntax tree) for
// lossless parsing and serialization. Comments (line and block) are
// preserved through the format cycle.
//
// Formatting walks the CST to apply indentation and normalize spacing.
// This is idempotent by construction — the same tree always produces
// the same output regardless of original formatting.
//
// Trailing commas are added to expanded objects and arrays by default,
// matching Prettier's trailingComma: "all" behavior. Options.TrailingCommas
// can preserve the input style or remove trailing commas instead.
package jsoncfmt

import (
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats JSONC files using CST-based indentation.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for JSONC.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:    formatter.IndentSpaces,
		IndentWidth:    2,
		FinalNewline:   true,
		LineEnding:     formatter.LineEndingLF,
		SortKeys:       false,
		TrailingCommas: formatter.TrailingCommasAll,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid JSONC (JSON with comments and
// trailing commas).
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	v, err := hujson.Parse(src)
	if err != nil {
		return nil, err
	}

	// Detect before sorting: sorting moves the trailing comma marker off the
	// last member.
	fs := &formatState{
		indent:         buildIndent(opts),
		trailingCommas: wantTrailingCommas(&v, opts.TrailingCommas),
	}

	if opts.SortKeys {
		sortObject(&v)
	}

	fs.formatValue(&v, 0)

	out := v.Pack()

	// Ensure correct trailing newline.
	out = trimTrailingNewlines(out)
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	return out, nil
}

// formatState holds the configuration for a single format pass.
type formatState struct {
	indent         string // indent string (e.g., "  " or "\t")
	trailingCommas bool   // true if multiline collections get a trailing comma
}

// wantTrailingCommas resolves the trailing comma mode against the parsed input.
func wantTrailingCommas(v *hujson.Value, mode formatter.TrailingCommas) bool {
	switch mode {
	case formatter.TrailingCommasAll:
		return true
	case formatter.TrailingCommasNone:
		return false
	default:
		return hasTrailingComma(v)
	}
}

// hasTrailingComma reports whether any object or array in v already ends with
// a trailing comma. hujson leaves a non-nil AfterExtra on the last member of a
// collection exactly when a comma follows it.
func hasTrailingComma(v *hujson.Value) bool {
	switch val := v.Value.(type) {
	case *hujson.Object:
		if n := len(val.Members); n > 0 && val.Members[n-1].Value.AfterExtra != nil {
			return true
		}
		for i := range val.Members {
			if hasTrailingComma(&val.Members[i].Value) {
				return true
			}
		}
	case *hujson.Array:
		if n := len(val.Elements); n > 0 && val.Elements[n-1].AfterExtra != nil {
			return true
		}
		for i := range val.Elements {
			if hasTrailingComma(&val.Elements[i]) {
				return true
			}
		}
	default:
		// Literals hold no trailing comma.
	}
	return false
}

// formatValue applies indentation to a value node in the CST.
func (fs *formatState) formatValue(v *hujson.Value, depth int) {
	switch val := v.Value.(type) {
	case *hujson.Object:
		fs.formatObject(val, depth)
	case *hujson.Array:
		fs.formatArray(val, depth)
	case hujson.Literal:
		// Scalars need no structural formatting.
	default:
		// Unknown value type — leave unchanged.
	}
}

// formatObject applies indentation to an object's members.
func (fs *formatState) formatObject(obj *hujson.Object, depth int) {
	if len(obj.Members) == 0 {
		if hasComment(obj.AfterExtra) {
			// Comments in empty objects need proper indentation.
			childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
			closeIndent := "\n" + strings.Repeat(fs.indent, depth)
			obj.AfterExtra = reindentExtra(obj.AfterExtra, childIndent)
			// Ensure closing brace is on its own line after the comment.
			s := string(obj.AfterExtra)
			if !strings.HasSuffix(s, closeIndent) {
				obj.AfterExtra = hujson.Extra(s + closeIndent)
			}
		} else {
			obj.AfterExtra = clearWhitespace(obj.AfterExtra)
		}
		return
	}

	childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
	closeIndent := "\n" + strings.Repeat(fs.indent, depth)

	for i := range obj.Members {
		m := &obj.Members[i]

		// Preserve comments from BeforeExtra, apply correct indentation.
		m.Name.BeforeExtra = reindentExtra(m.Name.BeforeExtra, childIndent)
		m.Name.AfterExtra = nil

		// Single space between colon and value.
		m.Value.BeforeExtra = hujson.Extra(" ")
		m.Value.AfterExtra = clearWhitespace(m.Value.AfterExtra)

		// Recurse into nested structures.
		fs.formatValue(&m.Value, depth+1)
	}

	// Trailing comma on the last member, if enabled.
	last := &obj.Members[len(obj.Members)-1]
	if fs.trailingCommas {
		last.Value.AfterExtra = ensureTrailingComma(last.Value.AfterExtra)
	}

	obj.AfterExtra = reindentExtra(obj.AfterExtra, closeIndent)
	if obj.AfterExtra == nil {
		obj.AfterExtra = hujson.Extra(closeIndent)
	}
}

// formatArray applies indentation to an array's elements.
func (fs *formatState) formatArray(arr *hujson.Array, depth int) {
	if len(arr.Elements) == 0 {
		if hasComment(arr.AfterExtra) {
			childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
			closeIndent := "\n" + strings.Repeat(fs.indent, depth)
			arr.AfterExtra = reindentExtra(arr.AfterExtra, childIndent)
			s := string(arr.AfterExtra)
			if !strings.HasSuffix(s, closeIndent) {
				arr.AfterExtra = hujson.Extra(s + closeIndent)
			}
		} else {
			arr.AfterExtra = clearWhitespace(arr.AfterExtra)
		}
		return
	}

	// Keep short primitive arrays on one line.
	// Note: inlined arrays intentionally omit trailing commas.
	// A single-line array like [1, 2, 3] is cleaner without a trailing comma.
	if isInlineArray(arr) {
		for i := range arr.Elements {
			if i == 0 {
				arr.Elements[i].BeforeExtra = nil
			} else {
				arr.Elements[i].BeforeExtra = hujson.Extra(" ")
			}
			arr.Elements[i].AfterExtra = nil
		}
		arr.AfterExtra = nil
		return
	}

	// Expand to multiline.
	childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
	closeIndent := "\n" + strings.Repeat(fs.indent, depth)

	for i := range arr.Elements {
		arr.Elements[i].BeforeExtra = reindentExtra(arr.Elements[i].BeforeExtra, childIndent)
		arr.Elements[i].AfterExtra = clearWhitespace(arr.Elements[i].AfterExtra)
		fs.formatValue(&arr.Elements[i], depth+1)
	}

	// Trailing comma on the last element, if enabled.
	last := &arr.Elements[len(arr.Elements)-1]
	if fs.trailingCommas {
		last.AfterExtra = ensureTrailingComma(last.AfterExtra)
	}

	arr.AfterExtra = reindentExtra(arr.AfterExtra, closeIndent)
	if arr.AfterExtra == nil {
		arr.AfterExtra = hujson.Extra(closeIndent)
	}
}

// isInlineArray returns true if the array should stay on one line.
// Short arrays of only primitive values (no nested objects/arrays, no comments)
// are kept inline.
func isInlineArray(arr *hujson.Array) bool {
	// totalLen slightly over-counts (+2) because the last element has no
	// trailing comma+space. This conservative bias means arrays at exactly
	// the line limit are expanded rather than compacted.
	totalLen := 2 // [ and ]
	for _, el := range arr.Elements {
		if _, ok := el.Value.(hujson.Literal); !ok {
			return false
		}
		if hasComment(el.BeforeExtra) || hasComment(el.AfterExtra) {
			return false
		}
		totalLen += len(el.Value.(hujson.Literal)) + 2
	}
	return totalLen < 80
}

// hasComment returns true if the extra contains a comment.
func hasComment(extra hujson.Extra) bool {
	s := string(extra)
	return strings.Contains(s, "//") || strings.Contains(s, "/*")
}

// reindentExtra normalizes indentation in Extra (comment/whitespace) content.
// Blank lines between comments are collapsed — this matches prettier's behavior
// of not preserving blank lines within structures.
func reindentExtra(extra hujson.Extra, newIndent string) hujson.Extra {
	if extra == nil {
		return hujson.Extra(newIndent)
	}

	s := string(extra)

	// If extra is whitespace-only (no comments), replace entirely.
	if !hasComment(extra) {
		return hujson.Extra(newIndent)
	}

	// Has comments — preserve them with correct indentation.
	// Pattern: whitespace + comment content + whitespace
	// We need to re-indent each line.
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "" {
			continue
		}
		if trimmed == "" {
			continue
		}
		// This is a comment line — emit with new indentation.
		b.WriteString(newIndent)
		b.WriteString(trimmed)
	}
	b.WriteString(newIndent)

	return hujson.Extra(b.String())
}

// clearWhitespace removes whitespace from Extra but preserves comments.
func clearWhitespace(extra hujson.Extra) hujson.Extra {
	if extra == nil {
		return nil
	}
	if !hasComment(extra) {
		return nil
	}
	// Has a comment — preserve it (inline comments after values).
	s := string(extra)
	s = strings.TrimRight(s, " \t\n\r")
	// Ensure single space before inline comment.
	s = " " + strings.TrimLeft(s, " \t")
	return hujson.Extra(s)
}

// ensureTrailingComma ensures the AfterExtra signals a trailing comma.
// In hujson, a non-nil AfterExtra on the last member means a trailing comma
// is emitted.
func ensureTrailingComma(extra hujson.Extra) hujson.Extra {
	if extra == nil {
		return hujson.Extra("")
	}
	return extra
}

// sortObject recursively sorts object members by key name.
func sortObject(v *hujson.Value) {
	switch val := v.Value.(type) {
	case *hujson.Object:
		slices.SortStableFunc(val.Members, func(a, b hujson.ObjectMember) int {
			aKey := a.Name.Value.(hujson.Literal).String()
			bKey := b.Name.Value.(hujson.Literal).String()
			return strings.Compare(aKey, bKey)
		})
		for i := range val.Members {
			sortObject(&val.Members[i].Value)
		}
	case *hujson.Array:
		for i := range val.Elements {
			sortObject(&val.Elements[i])
		}
	default:
		// Literals and unknown types have no children to sort.
	}
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

// trimTrailingNewlines removes all trailing newline characters.
func trimTrailingNewlines(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
