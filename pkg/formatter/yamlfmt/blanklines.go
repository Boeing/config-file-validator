// Blank line normalization: collapses consecutive blank lines, strips blank lines
// after document markers and colons, and before end-of-document comments.
package yamlfmt

// collapseConsecutiveBlankLines removes excess blank lines from the token stream,
// keeping at most 1 consecutive blank line between content. This matches prettier's
// behavior. Block scalar content is unaffected because it's inside TokBlockScalar
// tokens (opaque).
func collapseConsecutiveBlankLines(tokens []Token) []Token {
	result := make([]Token, 0, len(tokens))
	consecutiveNewlines := 0

	for _, tok := range tokens {
		switch tok.Kind {
		case TokNewline:
			consecutiveNewlines++
			if consecutiveNewlines <= 2 {
				result = append(result, tok)
			}
		case TokIndent:
			// Indent tokens are transparent for blank-line counting.
			// A blank line is: newline + indent + newline.
			// Only suppress zero-width indents on excess blank lines.
			if consecutiveNewlines < 2 || len(tok.Raw) > 0 {
				result = append(result, tok)
			}
		case TokBlockScalar:
			// Block scalars end with newline(s). Count trailing newlines
			// so the consecutive-newline counter reflects reality.
			result = append(result, tok)
			trailingNL := countTrailingNewlines(tok.Raw)
			consecutiveNewlines = trailingNL
		default:
			consecutiveNewlines = 0
			result = append(result, tok)
		}
	}
	return result
}

// stripBlankLinesAfterDocMarkers removes blank lines immediately following
// document start (---) and document end (...) markers. prettier places exactly
// one newline between a document marker and the body content — never a blank line.
// Source: prettier v3.9.6 printer-yaml.js, "document" case uses join(hardline, parts).
func stripBlankLinesAfterDocMarkers(tokens []Token) []Token {
	result := make([]Token, 0, len(tokens))
	i := 0
	for i < len(tokens) {
		result = append(result, tokens[i])

		if tokens[i].Kind != TokDocStart && tokens[i].Kind != TokDocEnd {
			i++
			continue
		}

		// Expect TokNewline after the marker.
		j := i + 1
		if j >= len(tokens) || tokens[j].Kind != TokNewline {
			i++
			continue
		}
		result = append(result, tokens[j]) // keep the single newline
		j++

		// Skip blank lines: bare TokNewline or (TokIndent + TokNewline) pairs.
		j = skipBlankLines(tokens, j)
		i = j
	}
	return result
}

// stripBlankLinesAfterColon removes blank lines that appear between a mapping
// key's colon and its first child value. Prettier strips these unconditionally:
// a blank line is only valid between siblings, not between a key and its value.
//
// Pattern detected: TokColon → TokNewline → (blank: TokIndent? + TokNewline)+ → value
// The blank-line tokens (TokIndent + TokNewline pairs) are removed.
func stripBlankLinesAfterColon(tokens []Token) []Token {
	result := make([]Token, 0, len(tokens))
	i := 0
	for i < len(tokens) {
		result = append(result, tokens[i])

		if tokens[i].Kind != TokColon {
			i++
			continue
		}

		// Skip optional TokSpace between colon and newline.
		j := i + 1
		for j < len(tokens) && tokens[j].Kind == TokSpace {
			result = append(result, tokens[j])
			j++
		}
		// Must have a TokNewline after the colon.
		if j >= len(tokens) || tokens[j].Kind != TokNewline {
			i++
			continue
		}
		result = append(result, tokens[j])
		j++

		// Check if blank lines exist after the colon's newline.
		blankEnd := skipBlankLines(tokens, j)
		if blankEnd == j {
			// No blank lines — nothing to strip.
			i = j
			continue
		}

		// Blank lines found. Determine if the next content is a child (deeper
		// indent) or a sibling (same/shallower). Only strip for children.
		colonLineIndent := 0
		for k := i - 1; k >= 0; k-- {
			if tokens[k].Kind == TokIndent {
				colonLineIndent = len(tokens[k].Raw)
				break
			}
			if tokens[k].Kind == TokNewline {
				break
			}
		}

		nextIndent := 0
		if blankEnd < len(tokens) && tokens[blankEnd].Kind == TokIndent {
			nextIndent = len(tokens[blankEnd].Raw)
		}

		if nextIndent > colonLineIndent {
			// Child content — strip blank lines (colon's value follows).
			i = blankEnd
		} else {
			// Sibling or parent — preserve blank lines.
			i = j
		}
	}
	return result
}

// skipBlankLines advances past consecutive blank lines starting at position i.
// A blank line is either a bare TokNewline or a TokIndent followed by TokNewline.
func skipBlankLines(tokens []Token, i int) int {
	for i < len(tokens) {
		if tokens[i].Kind == TokNewline {
			i++
		} else if tokens[i].Kind == TokIndent && i+1 < len(tokens) && tokens[i+1].Kind == TokNewline {
			i += 2
		} else {
			break
		}
	}
	return i
}

// stripBlankLinesBeforeEndComments removes blank lines between document content
// and end-of-document trailing comments when the document body is a sequence.
// prettier v3.9.6 uses hardline (single newline) between sequence content and
// endComments, but preserves blank lines in mapping-body documents.
//
// prettier rule (printer-yaml.js, documentBody case, tag 3.9.6):
//
//	lastChild = children.at(-1)
//	shouldPreserveEmptyLine = isNode(lastChild, ["mapping"]) && isPreviousLineEmpty(...)
//	separator = shouldPreserveEmptyLine ? [hardline, hardline] : hardline
//
// Additionally, only col-0 comments are documentBody endComments in prettier's AST.
// Indented comments are sequenceItem endComments and preserve blank lines.
func stripBlankLinesBeforeEndComments(tokens []Token) []Token {
	// Determine if the LAST document body is a mapping (preserve) or sequence (strip).
	if documentBodyIsMapping(tokens) {
		return tokens
	}

	// Find the last non-comment, non-whitespace content token.
	lastContentIdx := -1
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i].Kind {
		case TokNewline, TokIndent, TokSpace, TokComment:
			continue
		default:
			lastContentIdx = i
		}
		if lastContentIdx >= 0 {
			break
		}
	}
	if lastContentIdx < 0 {
		return tokens
	}

	// Check if there are trailing comments after last content.
	commentIdx := -1
	for i := lastContentIdx + 1; i < len(tokens); i++ {
		if tokens[i].Kind == TokComment {
			commentIdx = i
			break
		}
	}
	if commentIdx < 0 {
		return tokens
	}

	// Check if the trailing comment is indented (item-level endComment).
	// In prettier's AST, indented comments belong to the sequenceItem, not
	// the documentBody. Only col-0 comments are documentBody endComments.
	for i := commentIdx - 1; i > lastContentIdx; i-- {
		if tokens[i].Kind == TokIndent && len(tokens[i].Raw) > 0 {
			return tokens // indented → item-level, preserve blank
		}
		if tokens[i].Kind == TokNewline {
			break // col-0 → document-level, proceed with stripping
		}
	}

	// Count newlines between last content and first trailing comment.
	newlineCount := 0
	for i := lastContentIdx + 1; i < commentIdx; i++ {
		if tokens[i].Kind == TokNewline {
			newlineCount++
		}
	}
	if newlineCount < 2 {
		return tokens // no blank line to strip
	}

	// Strip: rebuild with exactly 1 newline before the comment.
	result := make([]Token, 0, len(tokens))
	result = append(result, tokens[:lastContentIdx+1]...)
	result = append(result, Token{Kind: TokNewline, Raw: []byte("\n")})
	result = append(result, tokens[commentIdx:]...)
	return result
}

// documentBodyIsMapping returns true if the LAST document body's first content
// is a mapping key. In multi-doc files, trailing comments at end-of-stream
// belong to the last document. Scans from the last TokDocStart forward (or
// from the beginning if no TokDocStart exists).
func documentBodyIsMapping(tokens []Token) bool {
	// Find the last TokDocStart.
	scanStart := 0
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].Kind == TokDocStart {
			scanStart = i + 1
			break
		}
	}

	// Scan forward from scanStart to find the first structural token.
	for i := scanStart; i < len(tokens); i++ {
		switch tokens[i].Kind {
		case TokNewline, TokIndent, TokSpace, TokComment, TokDirective, TokDocEnd:
			continue
		case TokKey:
			return true
		default:
			return false
		}
	}
	return false
}

// countTrailingNewlines counts how many newlines are at the end of a byte slice.
func countTrailingNewlines(data []byte) int {
	count := 0
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == '\n' {
			count++
		} else if data[i] == '\r' {
			continue // part of \r\n
		} else {
			break
		}
	}
	return count
}
