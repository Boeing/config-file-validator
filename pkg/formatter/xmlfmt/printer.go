package xmlfmt

import (
	"bytes"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted applies formatting to the token stream.
func printFormatted(tokens []Token, opts formatter.Options, src []byte) []byte {
	if len(tokens) == 0 {
		return nil
	}

	indent := buildIndentString(opts)

	// Annotate tokens with depth and mixed-content info.
	annotate(tokens, src)

	// In "ignore" mode, insert formatting whitespace (newlines + indent).
	// In "preserve" mode, only modify existing indent tokens.
	if opts.XMLWhitespaceSensitivity == formatter.XMLWhitespaceIgnore {
		tokens = insertFormattingWhitespace(tokens, indent)
	} else {
		reindentExisting(tokens, indent)
	}

	// Apply self-closing space preference.
	applySelfClosingSpace(tokens, opts.XMLSelfClosingSpace)

	// Serialize.
	var buf bytes.Buffer
	for _, tok := range tokens {
		buf.Write(tok.Raw)
	}
	out := buf.Bytes()

	// Strip trailing whitespace from each line.
	out = stripTrailingWhitespace(out)

	// Final newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	return formatter.NormalizeLineEndings(out, opts.LineEnding)
}

// buildIndentString constructs the indent unit string from options.
func buildIndentString(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		width = 2
	}
	return strings.Repeat(" ", width)
}

// =============================================================================
// Annotation
// =============================================================================

// annotate sets Depth on TokIndent tokens based on tag nesting.
// Does NOT set Structural — that is determined by markContentWhitespace
// which uses surrounding context to distinguish structural whitespace
// (between tags) from content whitespace (adjacent to text).
func annotate(tokens []Token, _ []byte) {
	depth := 0
	for i := range tokens {
		tokens[i].Depth = -1
		switch tokens[i].Kind {
		case TokIndent:
			tokens[i].Depth = depth
		case TokOpenTag:
			depth++
		case TokCloseTag:
			if depth > 0 {
				depth--
			}
		default:
		}
	}

	// Second pass: adjust depth for indents that precede close tags.
	// An indent before a close tag should be at the PARENT's depth (one less
	// than the element's interior depth) since the close tag belongs to the
	// parent context.
	for i := range tokens {
		if tokens[i].Kind != TokIndent || tokens[i].Depth < 0 {
			continue
		}
		next := nextNonWhitespace(tokens, i)
		if next >= 0 && tokens[next].Kind == TokCloseTag && tokens[i].Depth > 0 {
			tokens[i].Depth--
		}
	}
}

// =============================================================================
// Content whitespace classification
// =============================================================================

// markContentWhitespace classifies each TokNewline and TokIndent token as either
// structural (between tags — safe to remove during reformatting) or content
// (adjacent to text content — must be preserved to avoid data loss).
//
// A whitespace token is content whitespace if:
//   - Its previous non-whitespace token is a non-empty TokText, OR
//   - Its next non-whitespace token is a non-empty TokText
//
// Exception: whitespace immediately before a TokCloseTag is always structural,
// even if preceded by text. The close tag's position is determined by
// insertFormattingWhitespace, not by source whitespace.
func markContentWhitespace(tokens []Token) {
	for i := range tokens {
		if tokens[i].Kind != TokNewline && tokens[i].Kind != TokIndent {
			continue
		}

		prev := prevNonWhitespace(tokens, i)
		next := nextNonWhitespace(tokens, i)

		prevIsText := prev >= 0 && isNonEmptyText(tokens[prev])
		nextIsText := next >= 0 && isNonEmptyText(tokens[next])
		nextIsCloseTag := next >= 0 && tokens[next].Kind == TokCloseTag

		// Structural if: not adjacent to any text, OR immediately before a close tag.
		// Content (Structural=false) if: adjacent to text AND not before a close tag.
		tokens[i].Structural = (!prevIsText && !nextIsText) || nextIsCloseTag
	}
}

// prevNonWhitespace returns the index of the nearest preceding token that is
// not TokNewline or TokIndent. Returns -1 if none found.
func prevNonWhitespace(tokens []Token, idx int) int {
	for j := idx - 1; j >= 0; j-- {
		if tokens[j].Kind != TokNewline && tokens[j].Kind != TokIndent {
			return j
		}
	}
	return -1
}

// nextNonWhitespace returns the index of the nearest following token that is
// not TokNewline or TokIndent. Returns -1 if none found.
func nextNonWhitespace(tokens []Token, idx int) int {
	for j := idx + 1; j < len(tokens); j++ {
		if tokens[j].Kind != TokNewline && tokens[j].Kind != TokIndent {
			return j
		}
	}
	return -1
}

// isNonEmptyText returns true if the token is TokText with non-whitespace content.
func isNonEmptyText(tok Token) bool {
	return tok.Kind == TokText && strings.TrimSpace(string(tok.Raw)) != ""
}

// =============================================================================
// Ignore mode: insert formatting whitespace
// =============================================================================

// insertFormattingWhitespace restructures tokens for pretty-printed output.
// Removes whitespace-only text between tags, inserts proper newlines + indent.
// Mixed-content elements (containing both text and child elements) are emitted
// inline — no formatting whitespace is inserted within them.
func insertFormattingWhitespace(tokens []Token, indentUnit string) []Token {
	// First: remove structural whitespace, preserving content whitespace.
	cleaned := removeInsignificantWhitespace(tokens)

	// Second: insert newlines and indentation.
	var result []Token
	depth := 0
	i := 0

	// Track which depths have content newlines (for close tag formatting).
	// When a content TokNewline is emitted inside an element, that element's
	// close tag needs its own line (not inline after the last text).
	multilineAtDepth := make(map[int]bool)

	for i < len(cleaned) {
		tok := cleaned[i]

		switch tok.Kind { //nolint:revive // branches are intentionally parallel for readability per token type
		case TokOpenTag:
			// Newline + indent before open tag (except at depth 0, first element).
			if depth > 0 || (i > 0 && needsNewlineBefore(cleaned, i)) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)
			depth++

			// Check if this element has mixed content.
			closeIdx := findMatchingClose(cleaned, i)
			if closeIdx > 0 && isMixedContent(cleaned, i, closeIdx) {
				// Emit everything between open and close INLINE (no formatting).
				for j := i + 1; j <= closeIdx; j++ {
					result = append(result, cleaned[j])
				}
				// Adjust depth — the close tag decremented it.
				depth--
				i = closeIdx + 1
				continue
			}

		case TokCloseTag:
			depth--
			// Newline + indent before close tag if:
			// - previous meaningful token was a tag (element-only content), OR
			// - the element had multiline text content (content newlines emitted).
			if i > 0 && (prevIsTag(cleaned, i) || multilineAtDepth[depth+1]) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)
			delete(multilineAtDepth, depth+1) // clean up

		case TokSelfClose:
			if depth > 0 || (i > 0 && needsNewlineBefore(cleaned, i)) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)

		case TokComment, TokProcInst, TokCDATA:
			// Skip newline insertion if content whitespace already positioned this token.
			lastIsIndent := len(result) > 0 && result[len(result)-1].Kind == TokIndent
			if !lastIsIndent && (depth > 0 || (i > 0 && needsNewlineBefore(cleaned, i))) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)

		case TokXMLDecl, TokDoctype:
			result = append(result, tok)

		case TokText:
			// Keep text inline (no newline before it).
			result = append(result, tok)

		case TokNewline:
			// Content newline (inside text-only element) — emit as-is.
			result = append(result, tok)
			multilineAtDepth[depth] = true

		case TokIndent:
			// Content indent (inside text-only element) — reindent to current depth.
			tok.Raw = []byte(strings.Repeat(indentUnit, depth))
			result = append(result, tok)

		default:
			result = append(result, tok)
		}
		i++
	}

	return result
}

// findMatchingClose finds the index of the matching TokCloseTag for an open tag at idx.
// Returns -1 if not found.
func findMatchingClose(tokens []Token, openIdx int) int {
	depth := 0
	for j := openIdx; j < len(tokens); j++ {
		switch tokens[j].Kind {
		case TokOpenTag:
			depth++
		case TokCloseTag:
			depth--
			if depth == 0 {
				return j
			}
		default:
		}
	}
	return -1
}

// isMixedContent returns true if the element between openIdx and closeIdx
// contains BOTH non-whitespace text AND child element tokens.
func isMixedContent(tokens []Token, openIdx, closeIdx int) bool {
	hasText := false
	hasChild := false

	// Only check direct children — not deeply nested content.
	// We track depth relative to the parent element.
	depth := 0
	for j := openIdx + 1; j < closeIdx; j++ {
		switch tokens[j].Kind {
		case TokOpenTag:
			if depth == 0 {
				hasChild = true
			}
			depth++
		case TokCloseTag:
			depth--
		case TokSelfClose:
			if depth == 0 {
				hasChild = true
			}
		case TokText:
			if depth == 0 && strings.TrimSpace(string(tokens[j].Raw)) != "" {
				hasText = true
			}
		default:
		}
		if hasText && hasChild {
			return true
		}
	}
	return false
}

// removeInsignificantWhitespace removes structural whitespace (between tags)
// while preserving content whitespace (adjacent to text). Also removes
// whitespace-only TokText tokens (indentation artifacts between tags).
//
// Must be called AFTER annotate() so that Depth is set on TokIndent tokens.
func removeInsignificantWhitespace(tokens []Token) []Token {
	// Classify each whitespace token as structural or content.
	markContentWhitespace(tokens)

	var result []Token
	for _, tok := range tokens {
		switch tok.Kind {
		case TokIndent, TokNewline:
			if tok.Structural {
				continue // structural whitespace — remove
			}
			result = append(result, tok) // content whitespace — preserve
		case TokText:
			// Keep only non-whitespace text.
			if strings.TrimSpace(string(tok.Raw)) != "" {
				result = append(result, tok)
			}
		default:
			result = append(result, tok)
		}
	}
	return result
}

// appendNewlineIndent appends a newline token and an indent token.
func appendNewlineIndent(tokens []Token, depth int, indentUnit string) []Token {
	tokens = append(tokens, Token{Kind: TokNewline, Raw: []byte("\n")})
	if depth > 0 {
		tokens = append(tokens, Token{Kind: TokIndent, Raw: []byte(strings.Repeat(indentUnit, depth))})
	}
	return tokens
}

// needsNewlineBefore returns true if a newline should be inserted before token at idx.
func needsNewlineBefore(tokens []Token, idx int) bool {
	if idx == 0 {
		return false
	}
	prev := tokens[idx-1]
	return prev.Kind == TokOpenTag || prev.Kind == TokCloseTag ||
		prev.Kind == TokSelfClose || prev.Kind == TokComment ||
		prev.Kind == TokXMLDecl || prev.Kind == TokDoctype || prev.Kind == TokProcInst
}

// prevIsTag returns true if the previous non-whitespace token is a tag.
func prevIsTag(tokens []Token, idx int) bool {
	for j := idx - 1; j >= 0; j-- {
		switch tokens[j].Kind {
		case TokIndent, TokNewline:
			continue
		case TokOpenTag, TokCloseTag, TokSelfClose, TokComment, TokCDATA, TokProcInst:
			return true
		default:
			return false
		}
	}
	return false
}

// =============================================================================
// Preserve mode: only modify existing indent
// =============================================================================

// reindentExisting modifies existing TokIndent tokens based on their depth.
// Does not insert or remove any tokens. Used in preserve mode.
func reindentExisting(tokens []Token, indentUnit string) {
	for i := range tokens {
		if tokens[i].Kind != TokIndent || tokens[i].Depth < 0 {
			continue
		}
		tokens[i].Raw = []byte(strings.Repeat(indentUnit, tokens[i].Depth))
	}
}

// =============================================================================
// Self-closing space
// =============================================================================

// applySelfClosingSpace ensures or removes space before /> in self-closing tags.
//
//nolint:revive // flag-parameter: wantSpace is a simple formatting toggle, not control coupling
func applySelfClosingSpace(tokens []Token, wantSpace bool) {
	for i := range tokens {
		if tokens[i].Kind != TokSelfClose {
			continue
		}
		raw := tokens[i].Raw
		if len(raw) < 3 {
			continue
		}
		// Find the /> at the end.
		endsWithSlashGt := len(raw) >= 2 && raw[len(raw)-2] == '/' && raw[len(raw)-1] == '>'
		if !endsWithSlashGt {
			continue
		}
		hasSpace := len(raw) >= 3 && raw[len(raw)-3] == ' '

		// Check if /> is on its own line (preceded by newline + indentation).
		// If so, the space is indentation, not a self-closing space — don't modify.
		isIndented := false
		if hasSpace {
			for j := len(raw) - 3; j >= 0; j-- {
				if raw[j] == '\n' || raw[j] == '\r' {
					isIndented = true
					break
				}
				if raw[j] != ' ' && raw[j] != '\t' {
					break
				}
			}
		}

		if wantSpace && !isIndented {
			// Ensure exactly one space before />: <tag  /> → <tag /> or <tag/> → <tag />
			trimEnd := len(raw) - 2 // position of '/'
			for trimEnd > 0 && raw[trimEnd-1] == ' ' {
				trimEnd--
			}
			// trimEnd now points past the last non-space before />.
			if trimEnd == len(raw)-2 {
				// No space at all — insert one.
				newRaw := make([]byte, 0, len(raw)+1)
				newRaw = append(newRaw, raw[:trimEnd]...)
				newRaw = append(newRaw, ' ', '/', '>')
				tokens[i].Raw = newRaw
			} else if trimEnd == len(raw)-3 {
				// Exactly one space — already correct.
			} else {
				// Multiple spaces — collapse to one.
				newRaw := make([]byte, 0, trimEnd+3)
				newRaw = append(newRaw, raw[:trimEnd]...)
				newRaw = append(newRaw, ' ', '/', '>')
				tokens[i].Raw = newRaw
			}
		} else if !wantSpace && hasSpace && !isIndented {
			// Remove all spaces before />: <tag   /> → <tag/>
			trimEnd := len(raw) - 2 // position of '/'
			for trimEnd > 0 && raw[trimEnd-1] == ' ' {
				trimEnd--
			}
			newRaw := make([]byte, 0, trimEnd+2)
			newRaw = append(newRaw, raw[:trimEnd]...)
			newRaw = append(newRaw, '/', '>')
			tokens[i].Raw = newRaw
		}
	}
}

// =============================================================================
// Utilities
// =============================================================================

func stripTrailingWhitespace(data []byte) []byte {
	var result []byte
	lineStart := 0
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			end := i
			for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
				end--
			}
			result = append(result, data[lineStart:end]...)
			result = append(result, '\n')
			lineStart = i + 1
		case '\r':
			end := i
			for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
				end--
			}
			result = append(result, data[lineStart:end]...)
			result = append(result, '\r')
			if i+1 < len(data) && data[i+1] == '\n' {
				result = append(result, '\n')
				i++
			}
			lineStart = i + 1
		default:
		}
	}
	if lineStart < len(data) {
		end := len(data)
		for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
			end--
		}
		result = append(result, data[lineStart:end]...)
	}
	return result
}
