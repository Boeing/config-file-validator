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

	resolved := resolveOptions(opts)

	prettyOpts := &pretty.Options{
		Width:    resolved.MaxLineWidth,
		Prefix:   "",
		Indent:   indentString(resolved),
		SortKeys: resolved.SortKeys,
	}

	result := pretty.PrettyOptions(src, prettyOpts)
	result = preserveMemberBlankLines(src, result)

	// pretty always appends a trailing newline. Strip it if FinalNewline is false.
	if !resolved.FinalNewline && len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return formatter.NormalizeLineEndings(result, resolved.LineEnding), nil
}

// preserveMemberBlankLines restores blank lines between sibling members and
// elements after tidwall/pretty has normalized the JSON structure. Whitespace
// before closing delimiters is intentionally not copied.
func preserveMemberBlankLines(src, formatted []byte) []byte {
	source, err := hujson.Parse(src)
	if err != nil {
		return formatted
	}

	output, err := hujson.Parse(formatted)
	if err != nil {
		return formatted
	}

	copyMemberBlankLines(&source, &output)
	return output.Pack()
}

func copyMemberBlankLines(source, output *hujson.Value) {
	switch sourceValue := source.Value.(type) {
	case *hujson.Object:
		outputValue, ok := output.Value.(*hujson.Object)
		if !ok {
			return
		}

		sourceMembers := make(map[string][]int, len(sourceValue.Members))
		for i := range sourceValue.Members {
			key := objectMemberKey(&sourceValue.Members[i])
			sourceMembers[key] = append(sourceMembers[key], i)
		}

		for i := range outputValue.Members {
			outputMember := &outputValue.Members[i]
			key := objectMemberKey(outputMember)
			matches := sourceMembers[key]
			if len(matches) == 0 {
				continue
			}

			sourceIndex := matches[0]
			sourceMembers[key] = matches[1:]
			sourceMember := &sourceValue.Members[sourceIndex]

			if i > 0 && sourceIndex > 0 {
				outputMember.Name.BeforeExtra = preserveBlankLinePrefix(
					sourceMember.Name.BeforeExtra,
					outputMember.Name.BeforeExtra,
				)
			}
			copyMemberBlankLines(&sourceMember.Value, &outputMember.Value)
		}
	case *hujson.Array:
		outputValue, ok := output.Value.(*hujson.Array)
		if !ok {
			return
		}

		count := min(len(sourceValue.Elements), len(outputValue.Elements))
		for i := range count {
			if i > 0 {
				outputValue.Elements[i].BeforeExtra = preserveBlankLinePrefix(
					sourceValue.Elements[i].BeforeExtra,
					outputValue.Elements[i].BeforeExtra,
				)
			}
			copyMemberBlankLines(&sourceValue.Elements[i], &outputValue.Elements[i])
		}
	default:
		// Literals have no member whitespace to preserve.
	}
}

func objectMemberKey(member *hujson.ObjectMember) string {
	literal, ok := member.Name.Value.(hujson.Literal)
	if !ok {
		return ""
	}
	return string(literal)
}

func preserveBlankLinePrefix(source, output hujson.Extra) hujson.Extra {
	lineBreaks := strings.Count(string(source), "\n")
	if lineBreaks < 2 {
		return output
	}

	outputString := string(output)
	lastLineBreak := strings.LastIndexByte(outputString, '\n')
	if lastLineBreak < 0 {
		return output
	}

	indent := outputString[lastLineBreak+1:]
	return hujson.Extra(strings.Repeat("\n", lineBreaks) + indent)
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
