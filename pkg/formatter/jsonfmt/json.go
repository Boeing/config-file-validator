// Package jsonfmt provides a Formatter for JSON files.
//
// The formatter uses github.com/tidwall/pretty for canonical output.
// JSON has no comments, so comment preservation is not applicable.
//
// Defaults:
//   - 2-space indentation
//   - original key order preserved
//   - trailing newline
package jsonfmt

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/tailscale/hujson"
	"github.com/tidwall/pretty"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
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
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid JSON.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if !json.Valid(src) {
		return nil, errors.New("json: invalid JSON input")
	}

	original, err := hujson.Parse(src)
	if err != nil {
		return nil, err
	}

	resolved := resolveOptions(opts)

	prettyOpts := &pretty.Options{
		Width:    resolved.MaxLineWidth,
		Prefix:   "",
		Indent:   indentString(resolved),
		SortKeys: resolved.SortKeys,
	}

	result := pretty.PrettyOptions(src, prettyOpts)
	formatted, err := hujson.Parse(result)
	if err != nil {
		return nil, err
	}
	restoreBlankLines(&original, &formatted, prettyOpts.Indent, 0)
	result = formatted.Pack()

	// pretty always appends a trailing newline. Strip it if FinalNewline is false.
	if !resolved.FinalNewline && len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return formatter.NormalizeLineEndings(result, resolved.LineEnding), nil
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
	return opts
}

func indentString(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		width = 2 //nolint:mnd // JSON default
	}
	indent := make([]byte, width)
	for i := range indent {
		indent[i] = ' '
	}
	return string(indent)
}

// restoreBlankLines transfers blank-line boundaries from the original CST to
// the canonical output while leaving all other whitespace under pretty's
// control. Blank lines attach to the following object member or array element.
func restoreBlankLines(original, formatted *hujson.Value, indent string, depth int) {
	switch originalValue := original.Value.(type) {
	case *hujson.Object:
		formattedValue, ok := formatted.Value.(*hujson.Object)
		if !ok {
			return
		}
		originalIndexes := make(map[string][]int, len(originalValue.Members))
		for i := range originalValue.Members {
			key := objectMemberKey(&originalValue.Members[i])
			originalIndexes[key] = append(originalIndexes[key], i)
		}
		occurrences := make(map[string]int, len(originalIndexes))
		for i := range formattedValue.Members {
			key := objectMemberKey(&formattedValue.Members[i])
			sourceIndex := originalIndexes[key][occurrences[key]]
			occurrences[key]++
			sourceMember := &originalValue.Members[sourceIndex]
			formattedMember := &formattedValue.Members[i]
			if i > 0 && hasBlankLine(sourceMember.Name.BeforeExtra) {
				formattedMember.Name.BeforeExtra = addBlankLine(formattedMember.Name.BeforeExtra)
			}
			restoreBlankLines(&sourceMember.Value, &formattedMember.Value, indent, depth+1)
		}
	case *hujson.Array:
		formattedValue, ok := formatted.Value.(*hujson.Array)
		if !ok {
			return
		}
		hasBlankBoundary := false
		for i := 1; i < len(originalValue.Elements); i++ {
			if hasBlankLine(originalValue.Elements[i].BeforeExtra) {
				hasBlankBoundary = true
				break
			}
		}
		if hasBlankBoundary {
			childIndent := "\n" + strings.Repeat(indent, depth+1)
			closeIndent := "\n" + strings.Repeat(indent, depth)
			for i := range formattedValue.Elements {
				formattedValue.Elements[i].BeforeExtra = hujson.Extra(childIndent)
				if i > 0 && hasBlankLine(originalValue.Elements[i].BeforeExtra) {
					formattedValue.Elements[i].BeforeExtra = addBlankLine(formattedValue.Elements[i].BeforeExtra)
				}
			}
			formattedValue.AfterExtra = hujson.Extra(closeIndent)
		}
		for i := range originalValue.Elements {
			restoreBlankLines(&originalValue.Elements[i], &formattedValue.Elements[i], indent, depth+1)
		}
	default:
		// Literals have no collection boundaries to restore.
	}
}

func objectMemberKey(member *hujson.ObjectMember) string {
	return member.Name.Value.(hujson.Literal).String()
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
