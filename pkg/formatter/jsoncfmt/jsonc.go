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
		MaxLineWidth:   80,
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

	// Resolve defaults for unset options.
	defaults := DefaultOptions()
	if opts.MaxLineWidth == 0 {
		opts.MaxLineWidth = defaults.MaxLineWidth
	}
	if opts.IndentWidth == 0 {
		opts.IndentWidth = defaults.IndentWidth
	}

	// Detect before sorting: sorting moves the trailing comma marker off the
	// last member.
	fs := &formatState{
		indent:               buildIndent(opts),
		maxLineWidth:         opts.MaxLineWidth,
		trailingCommas:       wantTrailingCommas(&v, opts.TrailingCommas),
		removeTrailingCommas: opts.TrailingCommas == formatter.TrailingCommasNone,
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

// FormatValue formats a pre-parsed hujson Value with the given options.
// This is the shared format engine used by both the JSON and JSONC formatters.
// The caller is responsible for parsing and validating the input.
func FormatValue(v *hujson.Value, opts formatter.Options) ([]byte, error) {
	fs := &formatState{
		indent:               buildIndent(opts),
		maxLineWidth:         opts.MaxLineWidth,
		trailingCommas:       wantTrailingCommas(v, opts.TrailingCommas),
		removeTrailingCommas: opts.TrailingCommas == formatter.TrailingCommasNone,
	}

	if opts.SortKeys {
		sortObject(v)
	}

	fs.formatValue(v, 0)

	out := v.Pack()

	out = trimTrailingNewlines(out)
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)

	return out, nil
}

// formatState holds the configuration for a single format pass.
type formatState struct {
	indent               string // indent string (e.g., "  " or "\t")
	maxLineWidth         int    // max line width for inline decisions (0 = unlimited)
	trailingCommas       bool   // true if multiline collections get a trailing comma
	removeTrailingCommas bool   // true if trailing commas must be explicitly removed
	keyPrefixLen         int    // length of key + ": " on current line (set by formatObject before recursing)
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
			obj.AfterExtra = reindentCompactExtra(obj.AfterExtra, childIndent)
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

	// Keep the object on one line if it was already inline in the source and
	// fits within maxLineWidth. Objects that were multiline stay multiline.
	if fs.isInlineObject(obj, depth*len(fs.indent)) {
		for i := range obj.Members {
			m := &obj.Members[i]
			m.Name.BeforeExtra = hujson.Extra(" ")
			m.Name.AfterExtra = nil
			m.Value.BeforeExtra = hujson.Extra(" ")
			m.Value.AfterExtra = nil
			formatInlineValue(&m.Value)
		}
		obj.AfterExtra = hujson.Extra(" ")
		return
	}

	childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
	closeIndent := "\n" + strings.Repeat(fs.indent, depth)

	for i := range obj.Members {
		m := &obj.Members[i]
		preserveBlank := i > 0 && hasBlankLine(m.Name.BeforeExtra)

		// Preserve comments from BeforeExtra, apply correct indentation.
		if i == 0 {
			m.Name.BeforeExtra = reindentCompactExtra(m.Name.BeforeExtra, childIndent)
		} else {
			m.Name.BeforeExtra = reindentExtra(m.Name.BeforeExtra, childIndent)
			if preserveBlank {
				m.Name.BeforeExtra = addBlankLine(m.Name.BeforeExtra)
			}
		}
		m.Name.AfterExtra = nil

		// Single space between colon and value.
		m.Value.BeforeExtra = hujson.Extra(" ")
		m.Value.AfterExtra = clearWhitespace(m.Value.AfterExtra)

		// Recurse into nested structures.
		// Set key prefix so inline checks account for the full line width:
		// indent + "key": value
		fs.keyPrefixLen = len(m.Name.Value.(hujson.Literal)) + 2 // key + ": "
		fs.formatValue(&m.Value, depth+1)
		fs.keyPrefixLen = 0
	}

	// Trailing comma on the last member, if enabled.
	last := &obj.Members[len(obj.Members)-1]
	if fs.trailingCommas {
		last.Value.AfterExtra = ensureTrailingComma(last.Value.AfterExtra)
	} else if fs.removeTrailingCommas {
		obj.AfterExtra = removeTrailingComma(&last.Value.AfterExtra, obj.AfterExtra)
	}

	if fs.removeTrailingCommas {
		obj.AfterExtra = reindentClosingExtra(obj.AfterExtra, closeIndent)
	} else {
		obj.AfterExtra = reindentCompactExtra(obj.AfterExtra, closeIndent)
	}
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
			arr.AfterExtra = reindentCompactExtra(arr.AfterExtra, childIndent)
			s := string(arr.AfterExtra)
			if !strings.HasSuffix(s, closeIndent) {
				arr.AfterExtra = hujson.Extra(s + closeIndent)
			}
		} else {
			arr.AfterExtra = clearWhitespace(arr.AfterExtra)
		}
		return
	}

	// Expand to multiline (one element per line).
	childIndent := "\n" + strings.Repeat(fs.indent, depth+1)
	closeIndent := "\n" + strings.Repeat(fs.indent, depth)

	for i := range arr.Elements {
		preserveBlank := i > 0 && hasBlankLine(arr.Elements[i].BeforeExtra)
		if i == 0 {
			arr.Elements[i].BeforeExtra = reindentCompactExtra(arr.Elements[i].BeforeExtra, childIndent)
		} else {
			arr.Elements[i].BeforeExtra = reindentExtra(arr.Elements[i].BeforeExtra, childIndent)
			if preserveBlank {
				arr.Elements[i].BeforeExtra = addBlankLine(arr.Elements[i].BeforeExtra)
			}
		}
		arr.Elements[i].AfterExtra = clearWhitespace(arr.Elements[i].AfterExtra)
		// Array elements are on their own lines — no key prefix.
		savedKeyPrefix := fs.keyPrefixLen
		fs.keyPrefixLen = 0
		if nested, ok := arr.Elements[i].Value.(*hujson.Array); ok &&
			fs.isInlineArrayElement(nested, (depth+1)*len(fs.indent)) {
			formatInlineValue(&arr.Elements[i])
		} else {
			fs.formatValue(&arr.Elements[i], depth+1)
		}
		fs.keyPrefixLen = savedKeyPrefix
	}

	// Trailing comma on the last element, if enabled.
	last := &arr.Elements[len(arr.Elements)-1]
	if fs.trailingCommas {
		last.AfterExtra = ensureTrailingComma(last.AfterExtra)
	} else if fs.removeTrailingCommas {
		arr.AfterExtra = removeTrailingComma(&last.AfterExtra, arr.AfterExtra)
	}

	if fs.removeTrailingCommas {
		arr.AfterExtra = reindentClosingExtra(arr.AfterExtra, closeIndent)
	} else {
		arr.AfterExtra = reindentCompactExtra(arr.AfterExtra, closeIndent)
	}
	if arr.AfterExtra == nil {
		arr.AfterExtra = hujson.Extra(closeIndent)
	}
}

// formatInlineValue recursively normalizes whitespace inside a value for
// single-line display. Objects get { } padding, empty arrays get no padding,
// and commas get a trailing space.
func formatInlineValue(v *hujson.Value) {
	switch val := v.Value.(type) {
	case *hujson.Object:
		for i := range val.Members {
			m := &val.Members[i]
			m.Name.BeforeExtra = hujson.Extra(" ")
			m.Name.AfterExtra = nil
			m.Value.BeforeExtra = hujson.Extra(" ")
			m.Value.AfterExtra = nil
			formatInlineValue(&m.Value)
		}
		if len(val.Members) > 0 {
			val.AfterExtra = hujson.Extra(" ")
		} else {
			val.AfterExtra = nil
		}
	case *hujson.Array:
		for i := range val.Elements {
			if i == 0 {
				val.Elements[i].BeforeExtra = nil
			} else {
				val.Elements[i].BeforeExtra = hujson.Extra(" ")
			}
			val.Elements[i].AfterExtra = nil
			formatInlineValue(&val.Elements[i])
		}
		val.AfterExtra = nil
	default:
		// Literals need no formatting.
	}
}

// inlineValueLength returns the single-line character length of a value,
// or -1 if the value cannot be represented on one line (contains comments,
// blank lines between elements, etc).
func inlineValueLength(v *hujson.Value) int {
	switch val := v.Value.(type) {
	case hujson.Literal:
		return len(val)
	case *hujson.Object:
		if len(val.Members) == 0 {
			return 2 // {}
		}
		// If the object was multiline in the source (any member has a newline
		// in BeforeExtra), it cannot be inlined. This matches prettier's behavior:
		// expanded objects stay expanded.
		for _, m := range val.Members {
			if strings.ContainsAny(string(m.Name.BeforeExtra), "\n\r") {
				return -1
			}
		}
		total := 4 // "{ " + " }"
		for i, m := range val.Members {
			if i > 0 {
				total += 2 // ", "
			}
			if hasComment(m.Name.BeforeExtra) || hasComment(m.Value.BeforeExtra) {
				return -1
			}
			if i > 0 && hasBlankLine(m.Name.BeforeExtra) {
				return -1
			}
			total += len(m.Name.Value.(hujson.Literal)) // key
			total += 2                                  // ": "
			inner := inlineValueLength(&m.Value)
			if inner < 0 {
				return -1
			}
			total += inner
		}
		return total
	case *hujson.Array:
		if len(val.Elements) == 0 {
			return 2 // []
		}
		return -1
	default:
		return -1
	}
}

// inlineArrayValueLength returns the single-line length of an array used as an
// array element. Property and root arrays never use this compact form.
func inlineArrayValueLength(arr *hujson.Array) int {
	if hasComment(arr.AfterExtra) {
		return -1
	}
	total := 2 // "[" + "]"
	for i := range arr.Elements {
		if i > 0 {
			total += 2 // ", "
		}
		element := &arr.Elements[i]
		if hasComment(element.BeforeExtra) || hasComment(element.AfterExtra) {
			return -1
		}

		var length int
		switch value := element.Value.(type) {
		case hujson.Literal, *hujson.Object:
			length = inlineValueLength(element)
		case *hujson.Array:
			length = inlineArrayValueLength(value)
		default:
			return -1
		}
		if length < 0 {
			return -1
		}
		total += length
	}
	return total
}

// isInlineObject returns true if the object should be kept on one line.
// Objects are only kept inline if they were ALREADY on a single line in the
// input AND they fit within the max line width. This matches prettier's
// behavior: it never collapses multiline objects, only expands inline objects
// that exceed the print width.
func (fs *formatState) isInlineObject(obj *hujson.Object, prefixLen int) bool {
	if fs.maxLineWidth <= 0 {
		return false
	}
	// If the object was multiline in the source (any member preceded by a
	// newline), never collapse it.
	for _, m := range obj.Members {
		if strings.ContainsAny(string(m.Name.BeforeExtra), "\n\r") {
			return false
		}
	}
	valLen := inlineValueLength(&hujson.Value{Value: obj})
	if valLen < 0 {
		return false
	}
	return prefixLen+fs.keyPrefixLen+valLen <= fs.maxLineWidth
}

func (fs *formatState) isInlineArrayElement(arr *hujson.Array, prefixLen int) bool {
	if fs.maxLineWidth <= 0 {
		return false
	}
	valLen := inlineArrayValueLength(arr)
	return valLen >= 0 && prefixLen+valLen <= fs.maxLineWidth
}

// hasComment returns true if the extra contains a comment.
func hasComment(extra hujson.Extra) bool {
	s := string(extra)
	return strings.Contains(s, "//") || strings.Contains(s, "/*")
}

// reindentClosingExtra preserves a comment that starts on the same line as
// the final value. Other comments and whitespace use the collection's closing
// indentation.
func reindentClosingExtra(extra hujson.Extra, closeIndent string) hujson.Extra {
	s := strings.TrimLeft(string(extra), " \t")
	if strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") {
		s = strings.TrimRight(s, " \t\r\n")
		return hujson.Extra(" " + s + closeIndent)
	}
	return reindentCompactExtra(extra, closeIndent)
}

// reindentExtra normalizes indentation in Extra (comment/whitespace) content.
// Blank lines between sibling members are preserved; collection callers reinsert
// at most one intentional blank line between items.
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

// reindentCompactExtra normalizes leading and closing trivia without
// preserving blank lines.
func reindentCompactExtra(extra hujson.Extra, newIndent string) hujson.Extra {
	if extra == nil || !hasComment(extra) {
		return hujson.Extra(newIndent)
	}
	return reindentExtra(extra, newIndent)
}

func hasBlankLine(extra hujson.Extra) bool {
	for i := 0; i < len(extra); i++ {
		if extra[i] != '\n' && extra[i] != '\r' {
			continue
		}
		if extra[i] == '\r' && i+1 < len(extra) && extra[i+1] == '\n' {
			i++
		}
		j := i + 1
		for j < len(extra) && (extra[j] == ' ' || extra[j] == '\t') {
			j++
		}
		if j < len(extra) && (extra[j] == '\n' || extra[j] == '\r') {
			return true
		}
	}
	return false
}

func addBlankLine(extra hujson.Extra) hujson.Extra {
	return hujson.Extra("\n" + string(extra))
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

// removeTrailingComma clears hujson's trailing-comma marker. Comments stored
// in the marker are moved into the collection's closing extra so they remain
// before the closing delimiter.
func removeTrailingComma(extra *hujson.Extra, collectionExtra hujson.Extra) hujson.Extra {
	if *extra == nil {
		return collectionExtra
	}

	result := slices.Concat(*extra, collectionExtra)
	*extra = nil
	return result
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
