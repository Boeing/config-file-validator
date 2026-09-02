// Flow expansion phase: expands long flow sequences/mappings to multi-line format.
package yamlfmt

import (
	"bytes"
	"strings"
)

// expandLongFlowSequences finds TokFlow tokens (flow collections starting
// with [ or {) that exceed maxLineWidth and expands them to multi-line format:
//
//	key:
//	  [
//	    "elem1",
//	    "elem2",
//	  ]
//
// Per prettier source (flow-mapping-sequence.js): when a flow collection group
// breaks after a mapping key, first try the whole collection on the value's
// next line. If it still exceeds printWidth, put each element on a separate
// line indented by tabWidth, with a trailing comma on the last element.
func expandLongFlowSequences(tokens []Token, maxWidth, indentWidth int) {
	for i := range tokens {
		if tokens[i].Kind != TokFlow {
			continue
		}
		raw := tokens[i].Raw
		if len(raw) == 0 {
			continue
		}

		// Determine bracket type.
		openBracket := raw[0]
		var closeBracket byte
		switch openBracket {
		case '[':
			closeBracket = ']'
		case '{':
			closeBracket = '}'
		default:
			continue // unknown flow type
		}

		// Compute current line width: find the column where this token starts.
		lineWidth := computeTokenLineStart(tokens, i) + len(raw)
		if lineWidth <= maxWidth {
			continue // fits, no expansion needed
		}

		// Parse flow collection content into elements.
		elements := splitFlowElements(raw)
		if len(elements) == 0 {
			continue
		}

		// Compute indentation for the expanded form.
		// Parent indent = preceding TokIndent width.
		parentIndent := 0
		// Check if the flow follows a TokColon or TokDash on the same line.
		// If so, the expansion goes on the next line (value-position expansion).
		// If not, the flow is inline (root-level or bare sequence item) and
		// the bracket stays at the current column.
		afterColon := false

		// A value has to be indented past its key, and anything between the
		// line indent and the key — a sequence indicator, say — moves the key
		// to the right. Track the key so its real column is used instead of the
		// line indent; otherwise a value expanded inside a sequence item lands
		// on the key's own column, where it is no longer that key's value and
		// the document stops parsing.
		keyIdx := -1
		for j := i - 1; j >= 0; j-- {
			if tokens[j].Kind == TokIndent {
				parentIndent = len(tokens[j].Raw)
				break
			}
			if tokens[j].Kind == TokNewline {
				break
			}
			if tokens[j].Kind == TokColon && !afterColon {
				afterColon = true
				keyIdx = j - 1
			}
		}

		keyIndent := parentIndent
		if keyIdx >= 0 && tokens[keyIdx].Kind != TokIndent && tokens[keyIdx].Kind != TokNewline {
			keyIndent = computeTokenLineStart(tokens, keyIdx)
		}

		// Prettier gives a value-position flow collection the extra room gained
		// by moving it below the key before deciding to split its elements.
		// Preserve already-multiline collections instead of trying to compact
		// them through this single-line layout path.
		if afterColon && !bytes.ContainsAny(raw, "\r\n") &&
			keyIndent+indentWidth+len(raw) <= maxWidth {
			bracketIndent := strings.Repeat(" ", keyIndent+indentWidth)
			moved := make([]byte, 0, 1+len(bracketIndent)+len(raw))
			moved = append(moved, '\n')
			moved = append(moved, bracketIndent...)
			moved = append(moved, raw...)
			tokens[i].Raw = moved
			continue
		}

		var expanded []byte
		if afterColon {
			// Value-position: key: [long] → key:\n  [\n    elem,\n  ]
			// Bracket on next line at keyIndent + indentWidth,
			// elements at keyIndent + 2*indentWidth.
			bracketIndent := strings.Repeat(" ", keyIndent+indentWidth)
			elemIndent := strings.Repeat(" ", keyIndent+2*indentWidth)

			expanded = append(expanded, '\n')
			expanded = append(expanded, bracketIndent...)
			expanded = append(expanded, openBracket)
			for _, elem := range elements {
				expanded = append(expanded, '\n')
				expanded = append(expanded, elemIndent...)
				expanded = append(expanded, bytes.TrimSpace(elem)...)
				expanded = append(expanded, ',')
			}
			expanded = append(expanded, '\n')
			expanded = append(expanded, bracketIndent...)
			expanded = append(expanded, closeBracket)
		} else {
			// Inline-position: root-level or after dash.
			// Bracket stays at parentIndent (current column),
			// elements at parentIndent + indentWidth.
			bracketIndent := strings.Repeat(" ", parentIndent)
			elemIndent := strings.Repeat(" ", parentIndent+indentWidth)

			expanded = append(expanded, openBracket)
			for _, elem := range elements {
				expanded = append(expanded, '\n')
				expanded = append(expanded, elemIndent...)
				expanded = append(expanded, bytes.TrimSpace(elem)...)
				expanded = append(expanded, ',')
			}
			expanded = append(expanded, '\n')
			expanded = append(expanded, bracketIndent...)
			expanded = append(expanded, closeBracket)
		}

		tokens[i].Raw = expanded
	}
}

// computeTokenLineStart returns the column position where token i starts
// on its current line (bytes since last newline).
func computeTokenLineStart(tokens []Token, idx int) int {
	col := 0
	for j := idx - 1; j >= 0; j-- {
		if tokens[j].Kind == TokNewline {
			break
		}
		col += len(tokens[j].Raw)
	}
	return col
}

// splitFlowElements splits a flow sequence's raw bytes into individual element
// byte slices. Splits on comma at depth 0, skipping the outer [ and ].
func splitFlowElements(raw []byte) [][]byte {
	if len(raw) < 2 || (raw[0] != '[' && raw[0] != '{') {
		return nil
	}
	// Find closing bracket.
	inner := raw[1:]
	closeIdx := -1
	depth := 1
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		case '"':
			// Skip quoted string.
			for i++; i < len(inner) && inner[i] != '"'; i++ {
				if inner[i] == '\\' {
					i++
				}
			}
		case '\'':
			// Skip single-quoted string.
			for i++; i < len(inner) && inner[i] != '\''; i++ {
				if inner[i] == '\\' {
					i++
				}
			}
		default:
			// Other characters — no special handling.
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		closeIdx = len(inner)
	}
	content := inner[:closeIdx]

	// Split by comma at depth 0.
	var elements [][]byte
	var current []byte
	depth = 0
	for i := 0; i < len(content); i++ {
		b := content[i]
		switch b {
		case '[', '{':
			depth++
			current = append(current, b)
		case ']', '}':
			depth--
			current = append(current, b)
		case ',':
			if depth == 0 {
				if len(bytes.TrimSpace(current)) > 0 {
					elements = append(elements, current)
				}
				current = nil
			} else {
				current = append(current, b)
			}
		case '"':
			current = append(current, b)
			for i++; i < len(content) && content[i] != '"'; i++ {
				if content[i] == '\\' {
					current = append(current, content[i])
					i++
				}
				if i < len(content) {
					current = append(current, content[i])
				}
			}
			if i < len(content) {
				current = append(current, content[i])
			}
		case '\'':
			current = append(current, b)
			for i++; i < len(content) && content[i] != '\''; i++ {
				current = append(current, content[i])
			}
			if i < len(content) {
				current = append(current, content[i])
			}
		default:
			current = append(current, b)
		}
	}
	if len(bytes.TrimSpace(current)) > 0 {
		elements = append(elements, current)
	}
	return elements
}
