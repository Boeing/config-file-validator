// Quote style phase: converts quoted scalars between single and double quotes.
package yamlfmt

import (
	"bytes"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

func applyQuoteStyle(tokens []Token, style formatter.QuoteStyle) {
	for i := range tokens {
		if tokens[i].Kind != TokKey && tokens[i].Kind != TokValue && tokens[i].Kind != TokFlow {
			continue
		}

		if tokens[i].Kind == TokKey || tokens[i].Kind == TokValue {
			// Block scalar quote conversion.
			raw := bytes.TrimRight(tokens[i].Raw, " \t")
			if len(raw) < 2 {
				continue
			}
			first, last := raw[0], raw[len(raw)-1]
			if first == '"' && last == '"' && isSimpleQuoted(raw, '"') {
				tokens[i].Raw = convertQuote(raw, style, '"')
			} else if first == '\'' && last == '\'' && isSimpleQuoted(raw, '\'') {
				tokens[i].Raw = convertQuote(raw, style, '\'')
			}
			continue
		}

		// TokFlow: convert quoted scalars inside flow collections.
		tokens[i].Raw = convertFlowQuotes(tokens[i].Raw, style)
	}
}

// isSimpleQuoted verifies that a raw value token is a straightforwardly quoted
// scalar — the first and last bytes are the matching quotes with actual content
// between them. Rejects edge cases like `"'` (escaped quote at boundary) or
// values where the quote character appears in ambiguous positions.
func isSimpleQuoted(raw []byte, quote byte) bool {
	if len(raw) < 2 || raw[0] != quote || raw[len(raw)-1] != quote {
		return false
	}
	// For the value to be simply quoted, the last quote must be the CLOSING
	// quote, not part of an escape sequence or content.
	// In single-quoted YAML: '' is an escape for literal '. The closing ' must
	// not be preceded by an odd number of quotes (which would make it part of
	// an escape pair).
	if quote == '\'' {
		// Count consecutive quotes at the end (before the final one).
		n := 0
		for j := len(raw) - 2; j > 0 && raw[j] == '\''; j-- {
			n++
		}
		// If odd number of quotes precede the final one, the last quote is
		// actually the second half of an '' escape — not a closing delimiter.
		if n%2 == 1 {
			return false
		}
	} else {
		// For double-quoted: check the last quote isn't escaped by a backslash.
		n := 0
		for j := len(raw) - 2; j > 0 && raw[j] == '\\'; j-- {
			n++
		}
		if n%2 == 1 {
			return false
		}
	}
	return true
}

func convertQuote(raw []byte, style formatter.QuoteStyle, currentQuote byte) []byte {
	content := raw[1 : len(raw)-1]

	// Don't convert if content has backslashes (escapes).
	if currentQuote == '"' {
		for j := 0; j < len(content); j++ {
			if content[j] == '\\' {
				if j+1 < len(content) && content[j+1] == '"' {
					j++
					continue
				}
				return raw
			}
		}
	} else {
		for _, b := range content {
			if b == '\\' {
				return raw
			}
		}
	}

	// Determine target quote.
	hasSingle, hasDouble := false, false
	for _, b := range content {
		switch b {
		case '\'':
			hasSingle = true
		case '"':
			hasDouble = true
		default:
		}
	}

	var targetQuote byte
	switch style {
	case formatter.QuoteSingle:
		if hasSingle && !hasDouble {
			targetQuote = '"'
		} else {
			targetQuote = '\''
		}
	case formatter.QuoteDouble:
		if hasDouble && !hasSingle {
			targetQuote = '\''
		} else {
			targetQuote = '"'
		}
	default:
		return raw
	}

	if targetQuote == currentQuote {
		return raw
	}

	// Convert.
	var out []byte
	out = append(out, targetQuote)
	if currentQuote == '"' && targetQuote == '\'' {
		for j := 0; j < len(content); j++ {
			if content[j] == '\\' && j+1 < len(content) && content[j+1] == '"' {
				out = append(out, '"')
				j++
			} else if content[j] == '\'' {
				out = append(out, '\'', '\'')
			} else {
				out = append(out, content[j])
			}
		}
	} else if currentQuote == '\'' && targetQuote == '"' {
		for j := 0; j < len(content); j++ {
			if content[j] == '\'' && j+1 < len(content) && content[j+1] == '\'' {
				out = append(out, '\'')
				j++
			} else if content[j] == '"' {
				out = append(out, '\\', '"')
			} else {
				out = append(out, content[j])
			}
		}
	}
	out = append(out, targetQuote)
	return out
}

// convertFlowQuotes converts quoted scalars inside a flow collection token
// (TokFlow) to the target quote style. Walks the raw bytes, finds quoted
// scalar boundaries, and applies convertQuote to each one individually.
func convertFlowQuotes(raw []byte, style formatter.QuoteStyle) []byte {
	var result []byte
	i := 0
	for i < len(raw) {
		switch raw[i] {
		case '\'':
			// Find the end of this single-quoted scalar.
			end := findSingleQuoteEnd(raw, i)
			if end < 0 {
				// Malformed — copy rest verbatim.
				result = append(result, raw[i:]...)
				return result
			}
			scalar := raw[i : end+1]
			if isSimpleQuoted(scalar, '\'') {
				converted := convertQuote(scalar, style, '\'')
				result = append(result, converted...)
			} else {
				result = append(result, scalar...)
			}
			i = end + 1
		case '"':
			// Find the end of this double-quoted scalar.
			end := findDoubleQuoteEnd(raw, i)
			if end < 0 {
				result = append(result, raw[i:]...)
				return result
			}
			scalar := raw[i : end+1]
			if isSimpleQuoted(scalar, '"') {
				converted := convertQuote(scalar, style, '"')
				result = append(result, converted...)
			} else {
				result = append(result, scalar...)
			}
			i = end + 1
		default:
			result = append(result, raw[i])
			i++
		}
	}
	return result
}

// findSingleQuoteEnd finds the closing ' of a single-quoted YAML scalar.
// In single-quoted YAML, " is an escape for a literal '.
// Returns the index of the closing quote, or -1 if not found.
func findSingleQuoteEnd(raw []byte, start int) int {
	// start points to the opening '
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\'' {
			// Check if it's an escape ('')
			if i+1 < len(raw) && raw[i+1] == '\'' {
				i++ // skip the escape pair
				continue
			}
			return i
		}
	}
	return -1
}

// findDoubleQuoteEnd finds the closing " of a double-quoted YAML scalar.
// Handles backslash escapes (\" is not the end).
// Returns the index of the closing quote, or -1 if not found.
func findDoubleQuoteEnd(raw []byte, start int) int {
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\\' {
			i++ // skip escaped character
			continue
		}
		if raw[i] == '"' {
			return i
		}
	}
	return -1
}
