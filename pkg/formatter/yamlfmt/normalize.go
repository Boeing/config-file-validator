// Value and flow collection spacing normalization.
package yamlfmt

import (
	"bytes"
	"strings"
)

// normalizeValueSpacing strips leading horizontal whitespace from TokValue
// tokens that follow a TokColon. This normalizes "key:    value" to "key: value".
// The whitespace between : and the value is insignificant in YAML.
// Internal whitespace within the value is preserved.
func normalizeValueSpacing(tokens []Token) {
	for i := range tokens {
		if tokens[i].Kind != TokValue {
			continue
		}
		if i == 0 || tokens[i-1].Kind != TokColon {
			continue // Only strip values after colons, not continuation lines
		}
		tokens[i].Raw = bytes.TrimLeft(tokens[i].Raw, " \t")
		// Ensure the colon token ends with a space when the value is non-empty.
		// The tokenizer includes trailing space in TokColon only when the
		// original had a space — tabs or multiple spaces leave TokColon bare.
		if len(tokens[i].Raw) > 0 {
			colon := tokens[i-1].Raw
			if len(colon) == 0 || colon[len(colon)-1] != ' ' {
				tokens[i-1].Raw = append(colon, ' ')
			}
		}
	}
}

func normalizeSequenceSpacing(tokens []Token) {
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Kind == TokSpace && tokens[i-1].Kind == TokDash {
			tokens[i].Raw = nil
		}
	}
}

// normalizeFlowTokens normalizes comma separators and collection padding.
// Flow mappings ({...}) use bracketSpacing (space after { and before }), while
// flow sequences ([...]) do not. Commas use one following space unless they
// precede a line break or closing sequence bracket, matching Prettier's behavior.
func normalizeFlowTokens(tokens []Token) {
	for i := range tokens {
		if tokens[i].Kind != TokFlow {
			continue
		}
		tokens[i].Raw = normalizeFlowSpacing(tokens[i].Raw)
	}
}

func normalizeFlowSpacing(raw []byte) []byte {
	var out []byte
	quote := byte(0)
	escaped := false
	inComment := false
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if inComment {
			out = append(out, b)
			if b == '\n' || b == '\r' {
				inComment = false
			}
			continue
		}
		if quote != 0 {
			out = append(out, b)
			if quote == '"' && b == '\\' && !escaped {
				escaped = true
				continue
			}
			if b == quote && !escaped {
				if quote == '\'' && i+1 < len(raw) && raw[i+1] == '\'' {
					// Doubled single-quote: escape representing a literal '.
					// Consume BOTH bytes as content, stay in quoted state.
					out = append(out, raw[i+1])
					i++ // skip the second '
					continue
				}
				quote = 0
			}
			escaped = false
			continue
		}
		if b == '"' || b == '\'' {
			quote = b
			out = append(out, b)
			continue
		}
		if b == '#' && (i == 0 || raw[i-1] == ' ' || raw[i-1] == '\t' || raw[i-1] == '\n' || raw[i-1] == '\r') {
			inComment = true
			out = append(out, b)
			continue
		}
		if b == '!' && i+1 < len(raw) && raw[i+1] == '<' {
			if tagEnd := bytes.IndexByte(raw[i+2:], '>'); tagEnd >= 0 {
				tagEnd += i + 2
				out = append(out, raw[i:tagEnd+1]...)
				i = tagEnd
				continue
			}
		}
		switch b {
		case '{':
			out = append(out, b)
			if i+1 < len(raw) && raw[i+1] != '}' && raw[i+1] != ' ' && raw[i+1] != '\n' && raw[i+1] != '\r' {
				out = append(out, ' ')
			}
		case '}':
			if len(out) > 0 && out[len(out)-1] != '{' && out[len(out)-1] != ' ' && out[len(out)-1] != '\n' && out[len(out)-1] != '\r' {
				out = append(out, ' ')
			}
			out = append(out, b)
		case ',':
			out = append(out, b)
			for i+1 < len(raw) && (raw[i+1] == ' ' || raw[i+1] == '\t') {
				i++
			}
			if i+1 < len(raw) && raw[i+1] != '\n' && raw[i+1] != '\r' {
				out = append(out, ' ')
			}
		case '[':
			out = append(out, b)
			// Remove space after [ (prettier: no bracketSpacing for arrays).
			for i+1 < len(raw) && raw[i+1] == ' ' {
				i++
			}
		case ']':
			// Remove bracketSpacing before ]. Don't strip indentation after newline.
			trimEnd := len(out)
			for trimEnd > 0 && out[trimEnd-1] == ' ' {
				trimEnd--
			}
			if trimEnd == 0 || out[trimEnd-1] != '\n' {
				// Spaces preceded by non-newline: bracketSpacing, strip them.
				out = out[:trimEnd]
				if endsWithFlowTag(out) {
					// A tag with no explicit value still needs separation from ].
					out = append(out, ' ')
				}
			}
			// Else: spaces preceded by \n: indentation, preserve them.
			out = append(out, b)
		default:
			out = append(out, b)
		}
	}
	return out
}

// endsWithFlowTag reports whether the final token in a flow collection is a
// tag. YAML requires whitespace between a tag and a closing flow indicator.
func endsWithFlowTag(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}

	if raw[len(raw)-1] == '>' {
		start := bytes.LastIndex(raw, []byte("!<"))
		return start >= 0 && (start == 0 || strings.ContainsRune(" \t\n\r,[{", rune(raw[start-1])))
	}

	start := bytes.LastIndexAny(raw, " \t\n\r,[{") + 1
	return start < len(raw) && raw[start] == '!'
}
