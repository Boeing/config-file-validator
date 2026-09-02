// Reindentation phase: reads annotation fields (ASTDepth, InSeq, SeqOffset). Writes Token.Raw on TokIndent and TokBlockScalar.
package yamlfmt

import (
	"bytes"
	"strings"
)

// reindentTokens modifies TokIndent.Raw based on Structural + ASTDepth.
// Structural tokens with ASTDepth >= 0: new indent = computeNewIndent(...).
// Continuation tokens (ASTDepth < 0 or non-structural): shift by same delta as
// last structural token. Block scalars following a shifted indent also get
// their content shifted.
func reindentTokens(at AnnotatedTokens, targetWidth int, indentSequences bool) {
	tokens := at.Tokens()
	lastDelta := 0

	for i := range tokens {
		if tokens[i].Kind != TokIndent {
			continue
		}
		oldIndent := len(tokens[i].Raw)

		var newIndent int
		if tokens[i].Structural && tokens[i].ASTDepth >= 0 {
			hasDash := lineHasDash(tokens, i) || tokens[i].AtSeqItem
			newIndent = computeNewIndent(tokens[i].ASTDepth, tokens[i].InSeq,
				hasDash, tokens[i].SeqOffset, tokens[i].SequenceIndentDepth,
				targetWidth, indentSequences)
			lastDelta = newIndent - oldIndent
		} else {
			// Non-structural indent: shift by lastDelta.
			// Exceptions that stay at column 0:
			// - Blank-line indents (followed by TokNewline) — no visible whitespace on empty lines
			// - Document markers (--- and ...) — YAML spec requires column 0
			// - Plain-scalar continuations beginning with a document-marker prefix.
			//   Indenting these can make a trailing colon structural on the next parse.
			markerPrefixedScalar := false
			if oldIndent == 0 && i+1 < len(tokens) &&
				(tokens[i+1].Kind == TokKey || tokens[i+1].Kind == TokValue) {
				raw := tokens[i+1].Raw
				markerPrefixedScalar = bytes.HasPrefix(raw, []byte("---")) ||
					bytes.HasPrefix(raw, []byte("..."))
			}
			if i+1 < len(tokens) &&
				(tokens[i+1].Kind == TokNewline || tokens[i+1].Kind == TokDocStart ||
					tokens[i+1].Kind == TokDocEnd || markerPrefixedScalar) {
				newIndent = 0
			} else {
				newIndent = oldIndent + lastDelta
				if newIndent < 0 {
					newIndent = 0
				}
			}
		}

		tokens[i].Raw = []byte(strings.Repeat(" ", newIndent))

		// Normalize block scalar content indentation.
		// Target: parentIndent (newIndent) + targetWidth.
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Kind == TokNewline {
				break
			}
			if tokens[j].Kind == TokBlockScalar {
				if !hasExplicitIndentIndicator(tokens[j].Raw) {
					targetContentIndent := newIndent + targetWidth
					actualContentIndent := detectMinContentIndent(tokens[j].Raw)
					if actualContentIndent >= 0 && actualContentIndent != targetContentIndent {
						delta := targetContentIndent - actualContentIndent
						tokens[j].Raw = shiftBlockScalarIndent(tokens[j].Raw, delta)
					}
				} else {
					// Explicit indicator: just shift by the key's delta.
					delta := newIndent - oldIndent
					if delta != 0 {
						tokens[j].Raw = shiftBlockScalarIndent(tokens[j].Raw, delta)
					}
				}
				break
			}
		}
	}
}

// shiftBlockScalarIndent adjusts indentation of content lines within a block scalar.
func shiftBlockScalarIndent(raw []byte, delta int) []byte {
	var result []byte
	pos := 0

	// Copy header line (up to first newline).
	for pos < len(raw) {
		b := raw[pos]
		result = append(result, b)
		pos++
		if b == '\n' {
			break
		}
		if b == '\r' {
			if pos < len(raw) && raw[pos] == '\n' {
				result = append(result, raw[pos])
				pos++
			}
			break
		}
	}

	// Shift content lines.
	for pos < len(raw) {
		// Count leading spaces.
		spaces := 0
		for pos < len(raw) && raw[pos] == ' ' {
			spaces++
			pos++
		}
		// Apply delta.
		newSpaces := spaces + delta
		if newSpaces < 0 {
			newSpaces = 0
		}
		// Don't add indent to empty lines (would be trailing whitespace).
		if pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			result = append(result, []byte(strings.Repeat(" ", newSpaces))...)
		}

		// Copy rest of line + newline.
		for pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			result = append(result, raw[pos])
			pos++
		}
		if pos < len(raw) {
			result = append(result, raw[pos])
			pos++
			if raw[pos-1] == '\r' && pos < len(raw) && raw[pos] == '\n' {
				result = append(result, raw[pos])
				pos++
			}
		}
	}

	return result
}

// hasExplicitIndentIndicator checks if a block scalar header contains a
// digit (1-9) indicating an explicit indent level.
func hasExplicitIndentIndicator(raw []byte) bool {
	// Header is everything up to the first newline.
	for _, b := range raw {
		if b == '\n' || b == '\r' {
			break
		}
		if b >= '1' && b <= '9' {
			return true
		}
	}
	return false
}

// trimBlockScalarTrailingBlanks removes trailing blank lines from clip-chomped
// block scalars. For `|` (clip, the default), the scalar value ends with exactly
// one newline — any trailing blank lines are excess. For `|+` (keep), trailing
// blanks are preserved. For `|-` (strip), there are no trailing newlines at all.
//
// This matches prettier's behavior: clip-chomped scalars have trailing blanks removed.
func trimBlockScalarTrailingBlanks(tokens []Token) {
	for i := range tokens {
		if tokens[i].Kind != TokBlockScalar {
			continue
		}
		raw := tokens[i].Raw
		if len(raw) == 0 {
			continue
		}
		// Strip trailing whitespace from internal blank lines (whitespace-only
		// lines within block scalar content). This whitespace is never
		// semantically significant.
		raw = stripBlankLineWhitespace(raw)
		if blockScalarChomping(raw) == chompClip {
			raw = trimTrailingBlankLines(raw)
		}
		tokens[i].Raw = raw
	}
}

// chompMode represents YAML block scalar chomping behavior.
type chompMode int

const (
	chompClip  chompMode = iota // | (default) — single trailing newline
	chompStrip                  // |- — no trailing newline
	chompKeep                   // |+ — preserve all trailing newlines
)

// blockScalarChomping determines the chomping mode from a block scalar header.
func blockScalarChomping(raw []byte) chompMode {
	for _, b := range raw {
		if b == '\n' || b == '\r' {
			break
		}
		if b == '+' {
			return chompKeep
		}
		if b == '-' {
			return chompStrip
		}
	}
	return chompClip
}

// stripBlankLineWhitespace removes trailing spaces/tabs from blank lines
// (lines containing only whitespace) within block scalar content.
// Preserves content on non-blank lines. Skips the header line.
func stripBlankLineWhitespace(raw []byte) []byte {
	var result []byte
	pos := 0
	// Copy header line verbatim.
	for pos < len(raw) && raw[pos] != '\n' {
		result = append(result, raw[pos])
		pos++
	}
	if pos < len(raw) {
		result = append(result, raw[pos]) // include header \n
		pos++
	}
	// Process content lines.
	for pos < len(raw) {
		lineStart := pos
		// Scan to end of line.
		for pos < len(raw) && raw[pos] != '\n' {
			pos++
		}
		line := raw[lineStart:pos]
		// Check if line is blank (only spaces/tabs).
		isBlank := true
		for _, b := range line {
			if b != ' ' && b != '\t' {
				isBlank = false
				break
			}
		}
		if !isBlank {
			result = append(result, line...)
		}
		// Include the newline.
		if pos < len(raw) {
			result = append(result, raw[pos])
			pos++
		}
	}
	return result
}

// trimTrailingBlankLines removes excess trailing blank lines from a clip-chomped
// block scalar, keeping at most ONE trailing blank line after the last content line.
// The result has: ...lastContentLine\n\n (content newline + 1 blank line for separator).
// If the original only had 1 trailing newline (no blank lines), it stays as-is.
func trimTrailingBlankLines(raw []byte) []byte {
	// Find the position of the last non-empty line's ending newline.
	// Then keep at most one additional newline after it.
	lastContentEnd := -1 // position AFTER the last content char's newline
	pos := 0

	// Skip header line.
	for pos < len(raw) && raw[pos] != '\n' {
		pos++
	}
	if pos < len(raw) {
		pos++ // skip header \n
	}

	// Scan content lines, tracking the end of the last non-empty one.
	for pos < len(raw) {
		// Skip leading spaces.
		for pos < len(raw) && raw[pos] == ' ' {
			pos++
		}
		// Check if this is a non-empty line.
		if pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			// Non-empty line — advance to end of line.
			for pos < len(raw) && raw[pos] != '\n' {
				pos++
			}
			if pos < len(raw) {
				pos++ // include the \n
			}
			lastContentEnd = pos
		} else {
			// Empty/blank line — advance past it.
			if pos < len(raw) && raw[pos] == '\r' {
				pos++
			}
			if pos < len(raw) && raw[pos] == '\n' {
				pos++
			}
		}
	}

	if lastContentEnd < 0 {
		return raw // no content lines found
	}

	// After lastContentEnd: any remaining bytes are trailing blank lines.
	// Keep at most 1 additional newline (the inter-element blank line).
	remaining := raw[lastContentEnd:]
	if len(remaining) == 0 {
		return raw // no trailing blanks
	}
	// Keep one \n from the remaining (the blank line separator).
	return append(raw[:lastContentEnd], '\n')
}

// detectMinContentIndent returns the minimum leading-space count across
// all non-empty content lines in a block scalar (skipping the header line).
// Returns -1 if there are no non-empty content lines.
func detectMinContentIndent(raw []byte) int {
	// Skip header line.
	pos := 0
	for pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
		pos++
	}
	if pos < len(raw) {
		pos++ // skip \n
		if pos > 0 && raw[pos-1] == '\r' && pos < len(raw) && raw[pos] == '\n' {
			pos++
		}
	}

	minIndent := -1
	for pos < len(raw) {
		// Count leading spaces.
		spaces := 0
		for pos < len(raw) && raw[pos] == ' ' {
			spaces++
			pos++
		}
		// Check if line is non-empty.
		if pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			if minIndent < 0 || spaces < minIndent {
				minIndent = spaces
			}
		}
		// Skip to next line.
		for pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			pos++
		}
		if pos < len(raw) {
			pos++
			if pos > 0 && raw[pos-1] == '\r' && pos < len(raw) && raw[pos] == '\n' {
				pos++
			}
		}
	}
	return minIndent
}
