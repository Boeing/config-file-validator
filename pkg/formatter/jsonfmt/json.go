// Package jsonfmt provides a Formatter for JSON files.
//
// The formatter uses github.com/tailscale/hujson for canonical output.
// JSON is treated as a strict subset of JSONC — parsed with hujson,
// formatted with the shared JSONC format engine, trailing commas removed.
//
// Defaults:
//   - 2-space indentation
//   - original key order preserved
//   - trailing newline
//   - arrays/objects collapsed to one line when they fit within 80 columns
package jsonfmt

import (
	"encoding/json"
	"errors"

	"github.com/tailscale/hujson"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
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
		MaxLineWidth: 80,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid JSON.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	if !json.Valid(src) {
		return nil, errors.New("json: invalid JSON input")
	}

	v, err := hujson.Parse(src)
	if err != nil {
		return nil, err
	}

	// JSON forbids trailing commas — force removal.
	opts.TrailingCommas = formatter.TrailingCommasNone

	// Apply defaults for unset options.
	resolved := resolveOptions(opts)

	out, err := jsoncfmt.FormatValue(&v, resolved)
	if err != nil {
		return nil, err
	}

	// Normalize number literals: trim trailing zeros after decimal point.
	out = normalizeNumbers(out)
	return out, nil
}

// normalizeNumbers processes JSON output bytes and normalizes number literals
// to match prettier's printNumber behavior. Specifically:
// - Removes trailing zeros after decimal point (1.10 → 1.1, but 1.0 stays)
//
// Only processes numbers outside of string values.
func normalizeNumbers(data []byte) []byte {
	var result []byte
	i := 0
	for i < len(data) {
		// Skip strings (don't modify numbers inside string values).
		if data[i] == '"' {
			result = append(result, data[i])
			i++
			for i < len(data) && data[i] != '"' {
				if data[i] == '\\' {
					result = append(result, data[i])
					i++
					if i < len(data) {
						result = append(result, data[i])
						i++
					}
					continue
				}
				result = append(result, data[i])
				i++
			}
			if i < len(data) {
				result = append(result, data[i]) // closing "
				i++
			}
			continue
		}

		// Check for number literal (starts with digit or minus followed by digit).
		if isNumberStart(data, i) {
			numStart := i
			// Consume the full number literal.
			for i < len(data) && isNumberChar(data[i]) {
				i++
			}
			num := data[numStart:i]
			result = append(result, normalizeNumber(num)...)
			continue
		}

		result = append(result, data[i])
		i++
	}
	return result
}

// isNumberStart checks if position i starts a JSON number literal.
func isNumberStart(data []byte, i int) bool {
	if i >= len(data) {
		return false
	}
	if data[i] >= '0' && data[i] <= '9' {
		return true
	}
	if data[i] == '-' && i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '9' {
		return true
	}
	return false
}

// isNumberChar returns true for characters that can appear in a JSON number.
func isNumberChar(b byte) bool {
	return (b >= '0' && b <= '9') || b == '.' || b == '-' || b == '+' || b == 'e' || b == 'E'
}

// normalizeNumber applies prettier's printNumber rules to a single number literal.
// Rules (from prettier src/utilities/print-number.js):
// 1. Lowercase (E → e in scientific notation)
// 2a. Remove unnecessary + and leading zeros in exponent (e+034 → e34)
// 2b. Remove unnecessary scientific notation when exponent is 0 (1e0 → 1)
// 3. Remove trailing zeros after decimal point (1.10 → 1.1, but 1.0 stays)
func normalizeNumber(num []byte) []byte {
	// Rule 1: lowercase E → e
	result := make([]byte, len(num))
	copy(result, num)
	for i, b := range result {
		if b == 'E' {
			result[i] = 'e'
		}
	}

	// Rule 2a: normalize exponent (remove +, remove leading zeros)
	eIdx := -1
	for i, b := range result {
		if b == 'e' {
			eIdx = i
			break
		}
	}
	if eIdx >= 0 {
		// Parse exponent part: e[+-]?[0-9]+
		expStart := eIdx + 1
		sign := byte(0)
		if expStart < len(result) && (result[expStart] == '+' || result[expStart] == '-') {
			sign = result[expStart]
			expStart++
		}
		// Skip leading zeros in exponent (keep at least one digit).
		digitStart := expStart
		for digitStart < len(result)-1 && result[digitStart] == '0' {
			digitStart++
		}

		// Rule 2b: if the remaining exponent digits are all zeros, the
		// scientific notation is unnecessary (e.g. 1e0 → 1, 2e-0 → 2).
		// Matches prettier regex: /^([+-]?[\d.]+)e[+-]?0+$/ → "$1"
		allZeros := true
		for k := digitStart; k < len(result); k++ {
			if result[k] != '0' {
				allZeros = false
				break
			}
		}
		if allZeros && digitStart < len(result) {
			// Drop the entire exponent — the mantissa is the result.
			result = result[:eIdx]
		} else {
			// Rebuild exponent.
			var exp []byte
			exp = append(exp, 'e')
			if sign == '-' {
				exp = append(exp, '-')
			}
			// + is omitted (unnecessary)
			exp = append(exp, result[digitStart:]...)
			result = append(result[:eIdx], exp...)
		}
	}

	// Rule 3: trim trailing zeros after decimal point.
	dotIdx := -1
	for i, b := range result {
		if b == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx < 0 {
		return result
	}

	// Find end of decimal digits (before 'e' or end).
	decEnd := len(result)
	for i := dotIdx + 1; i < len(result); i++ {
		if result[i] == 'e' {
			decEnd = i
			break
		}
	}

	// Trim trailing zeros, keeping at least one digit after '.'.
	trimEnd := decEnd
	for trimEnd > dotIdx+2 && result[trimEnd-1] == '0' {
		trimEnd--
	}

	if trimEnd == decEnd {
		return result
	}

	// Rebuild: everything before trimEnd + everything from decEnd onward.
	var final []byte
	final = append(final, result[:trimEnd]...)
	final = append(final, result[decEnd:]...)
	return final
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
	if opts.MaxLineWidth == 0 {
		opts.MaxLineWidth = defaults.MaxLineWidth
	}
	return opts
}
