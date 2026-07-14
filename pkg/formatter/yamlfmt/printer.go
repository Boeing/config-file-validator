package yamlfmt

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted takes a token stream and formatting options, applies
// indent normalization and optional key sorting, then serializes.
func printFormatted(tokens []Token, opts formatter.Options, structuralLines map[int]bool) []byte {
	if len(tokens) == 0 {
		return nil
	}

	targetWidth := opts.IndentWidth
	if targetWidth <= 0 {
		targetWidth = 2
	}

	// Compute line numbers for each token.
	lineNums := computeLineNumbers(tokens)

	// Compute depth for each IndentToken (only using structural lines).
	depths := computeDepths(tokens, lineNums, structuralLines)

	// Sort keys if requested.
	if opts.SortKeys {
		tokens = sortKeys(tokens, depths, lineNums, structuralLines)
		// Recompute after sorting.
		lineNums = computeLineNumbers(tokens)
		depths = computeDepths(tokens, lineNums, structuralLines)
	}

	// Reindent: replace each IndentToken with normalized indent.
	tokens = applyIndent(tokens, depths, targetWidth, structuralLines)

	// Apply quote style preference if requested.
	if opts.QuoteStyle != formatter.QuotePreserve {
		tokens = applyQuoteStyle(tokens, opts.QuoteStyle)
	}

	// Normalize space before inline comments to exactly one space.
	// Only applies to inline comments (preceded by a value or key on the same line),
	// not to comments at line start.
	for i, tok := range tokens {
		if tok.Kind == TokSpace && i+1 < len(tokens) && tokens[i+1].Kind == TokComment {
			// Check that this space follows actual content (value, key, etc.), not indent.
			if i > 0 && tokens[i-1].Kind != TokIndent && tokens[i-1].Kind != TokNewline {
				tokens[i] = Token{Kind: TokSpace, Raw: []byte(" ")}
			}
		}
	}

	// Serialize: concatenate all Raw fields.
	var buf bytes.Buffer
	for _, tok := range tokens {
		buf.Write(tok.Raw)
	}

	out := buf.Bytes()

	// Strip trailing whitespace from each line.
	out = stripTrailingWhitespace(out)

	// Final newline handling.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)
	return out
}

// computeDepths returns the structural depth for each token in the stream.
// Only IndentTokens have meaningful depth values; other tokens get depth -1.
// Depth is computed by tracking indent level transitions:
//   - First non-zero indent seen sets the "unit" (typically 2)
//   - Each indent increase of one unit = +1 depth
//   - Depth stack tracks ancestor indent levels for correct dedent handling
func computeDepths(tokens []Token, lineNums []int, structuralLines map[int]bool) []int {
	depths := make([]int, len(tokens))
	for i := range depths {
		depths[i] = -1
	}

	// depthStack: stack of (indentLevel, depth) pairs.
	// Start with depth 0 at indent 0.
	type level struct {
		indent int
		depth  int
	}
	stack := []level{{0, 0}}

	for i, tok := range tokens {
		if tok.Kind != TokIndent {
			continue
		}

		// Skip indent on blank lines (indent followed immediately by newline).
		if i+1 < len(tokens) && tokens[i+1].Kind == TokNewline {
			continue
		}

		// Skip non-structural lines — they shouldn't affect the depth stack.
		line := lineNums[i]
		if structuralLines != nil && !structuralLines[line] {
			// Assign depth -1 (continuation) — handled specially by applyIndent.
			continue
		}

		indent := len(tok.Raw)

		// Find where this indent fits in the stack.
		// Pop until we find an ancestor with indent < current.
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		if indent > parent.indent {
			// Deeper — new child level.
			newDepth := parent.depth + 1
			stack = append(stack, level{indent, newDepth})
			depths[i] = newDepth
		} else {
			// Same level as parent (or below, which shouldn't happen after
			// the pop loop but handled gracefully) — same depth.
			depths[i] = parent.depth
		}
	}

	// Also assign depth 0 to positions where there's no indent token
	// but we're at the start of a line (implicit depth 0).
	// This is handled by the reindent function checking for depth -1.

	return depths
}

// applyIndent replaces each IndentToken's Raw with the correct number of spaces
// based on its computed depth and the target width.
// When a TokIndent precedes a TokBlockScalar, the block scalar's content
// lines are shifted by the same delta to maintain relative indentation.
// Only structural lines (keys, sequence items) get independently renormalized.
// Continuation lines (multi-line values) shift by the same delta as their parent.
func applyIndent(tokens []Token, depths []int, targetWidth int, structuralLines map[int]bool) []Token {
	result := make([]Token, len(tokens))
	copy(result, tokens)

	// Compute line number for each token.
	lineNums := computeLineNumbers(tokens)

	// Track the last delta applied to a structural indent.
	lastStructuralDelta := 0

	for i, tok := range result {
		if tok.Kind != TokIndent {
			continue
		}
		oldIndent := len(tok.Raw)
		line := lineNums[i]

		var newIndent int
		var delta int

		if depths[i] >= 0 {
			// Structural line: renormalize to depth × width.
			newIndent = depths[i] * targetWidth
			delta = newIndent - oldIndent
			lastStructuralDelta = delta
		} else {
			// Continuation line (depth -1): shift by same delta as parent.
			newIndent = oldIndent + lastStructuralDelta
			if newIndent < 0 {
				newIndent = 0
			}
			delta = newIndent - oldIndent
		}

		_ = line // used by structural check in computeDepths, kept for clarity

		result[i] = Token{
			Kind: TokIndent,
			Raw:  []byte(strings.Repeat(" ", newIndent)),
		}

		// If there's a block scalar on this line, shift its content indent.
		if delta != 0 {
			for j := i + 1; j < len(result); j++ {
				if result[j].Kind == TokNewline {
					break
				}
				if result[j].Kind == TokBlockScalar {
					result[j] = Token{
						Kind: TokBlockScalar,
						Raw:  shiftBlockScalarIndent(result[j].Raw, delta),
					}
					break
				}
			}
		}
	}

	return result
}

// computeLineNumbers assigns a 1-based line number to each token.
func computeLineNumbers(tokens []Token) []int {
	lines := make([]int, len(tokens))
	line := 1
	for i, tok := range tokens {
		lines[i] = line
		for _, b := range tok.Raw {
			if b == '\n' {
				line++
			}
		}
	}
	return lines
}

// shiftBlockScalarIndent adjusts the indentation of content lines within a
// block scalar by delta spaces. The header line is not modified.
// Positive delta adds spaces; negative delta removes spaces (clamped to 0).
func shiftBlockScalarIndent(raw []byte, delta int) []byte {
	// Split into lines. First line is the header (|, >, with indicators).
	var result []byte
	pos := 0

	// Copy header line (everything up to and including the first newline).
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

	// Process content lines: shift indentation.
	for pos < len(raw) {
		// Count leading spaces on this line.
		lineStart := pos
		spaces := 0
		for pos < len(raw) && raw[pos] == ' ' {
			spaces++
			pos++
		}

		// Calculate new indent for this line.
		newSpaces := spaces + delta
		if newSpaces < 0 {
			newSpaces = 0
		}

		// Apply shifted indent. Both empty/whitespace-only lines and
		// content lines get the same treatment.
		result = append(result, []byte(strings.Repeat(" ", newSpaces))...)

		// Copy the rest of the line (non-indent content + newline).
		_ = lineStart // suppress unused warning
		for pos < len(raw) && raw[pos] != '\n' && raw[pos] != '\r' {
			result = append(result, raw[pos])
			pos++
		}
		// Copy newline.
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

// applyQuoteStyle converts quoted scalars to the preferred quote style
// using prettier's conservative approach:
//   - If content has escape sequences (backslashes) → keep original style
//   - If content contains double quote → use single quotes
//   - If content contains single quote → use double quotes
//   - Otherwise → use preferred style
//
// This never performs escape sequence manipulation. It only swaps quotes
// when the raw content has no characters that would need escaping in the
// target style.
func applyQuoteStyle(tokens []Token, style formatter.QuoteStyle) []Token {
	result := make([]Token, len(tokens))
	copy(result, tokens)

	for i, tok := range result {
		if tok.Kind != TokValue {
			continue
		}
		raw := tok.Raw
		if len(raw) < 2 {
			continue
		}

		first := raw[0]
		last := raw[len(raw)-1]

		// Only process complete, single-line quoted scalars.
		// A complete quoted scalar starts and ends with the same quote character.
		// Multi-line quoted scalars (where the token doesn't end with the close quote)
		// must not be converted.
		if first == '"' && last == '"' && len(raw) >= 2 {
			result[i].Raw = convertQuote(raw, style, '"')
		} else if first == '\'' && last == '\'' && len(raw) >= 2 {
			result[i].Raw = convertQuote(raw, style, '\'')
		}
	}

	return result
}

// convertQuote applies the preferred quote style to a quoted scalar.
// currentQuote is the quote character currently wrapping the value.
func convertQuote(raw []byte, style formatter.QuoteStyle, currentQuote byte) []byte {
	content := raw[1 : len(raw)-1]

	// Multi-line scalars: don't touch (folding semantics differ).
	for _, b := range content {
		if b == '\n' || b == '\r' {
			return raw
		}
	}

	// Check for backslashes — if present, keep original style.
	// For double-quoted: \[^"] means real escapes → bail.
	// For single-quoted: any \ means user chose single to keep it literal → bail.
	if currentQuote == '"' {
		for j := 0; j < len(content); j++ {
			if content[j] == '\\' {
				if j+1 < len(content) && content[j+1] == '"' {
					j++ // skip \" — this is fine (just an escaped quote)
					continue
				}
				return raw // has non-quote escape → keep double
			}
		}
	} else {
		for _, b := range content {
			if b == '\\' {
				return raw // backslash in single-quoted → keep single
			}
		}
	}

	// Determine target quote.
	hasSingle := false
	hasDouble := false
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
			// Would need escaping in single → use double instead.
			targetQuote = '"'
		} else {
			targetQuote = '\''
		}
	case formatter.QuoteDouble:
		if hasDouble && !hasSingle {
			// Would need escaping in double → use single instead.
			targetQuote = '\''
		} else {
			targetQuote = '"'
		}
	default:
		return raw
	}

	// If target is same as current, nothing to do.
	if targetQuote == currentQuote {
		return raw
	}

	// Convert.
	var out []byte
	out = append(out, targetQuote)

	if currentQuote == '"' && targetQuote == '\'' {
		// Double → Single: remove \" escapes (they become literal ").
		for j := 0; j < len(content); j++ {
			if content[j] == '\\' && j+1 < len(content) && content[j+1] == '"' {
				out = append(out, '"')
				j++ // skip the escaped quote
			} else if content[j] == '\'' {
				out = append(out, '\'', '\'') // escape single quote
			} else {
				out = append(out, content[j])
			}
		}
	} else if currentQuote == '\'' && targetQuote == '"' {
		// Single → Double: replace '' with '.
		for j := 0; j < len(content); j++ {
			if content[j] == '\'' && j+1 < len(content) && content[j+1] == '\'' {
				out = append(out, '\'')
				j++ // skip the doubled quote
			} else if content[j] == '"' {
				out = append(out, '\\', '"') // escape double quote
			} else {
				out = append(out, content[j])
			}
		}
	}

	out = append(out, targetQuote)
	return out
}

// sortKeys sorts mapping entries within each scope by their key.
// An entry is: optional leading comments + key + colon + value/nested content.
// Entries are siblings if they share the same parent indent level.
func sortKeys(tokens []Token, depths []int, lineNums []int, structuralLines map[int]bool) []Token {
	// Sort at depth 0, then recursively sort nested scopes.
	return sortKeysAtDepth(tokens, depths, 0, 0, len(tokens), lineNums, structuralLines)
}

// sortKeysAtDepth sorts entries at the given target depth within [from, to),
// then recursively sorts within each entry's nested content.
func sortKeysAtDepth(tokens []Token, depths []int, targetDepth, from, to int, lineNums []int, structuralLines map[int]bool) []Token {
	entries := groupTopLevelEntries(tokens, depths, from, to, targetDepth, lineNums, structuralLines)

	if len(entries) >= 2 {
		tokens = sortEntrySlice(tokens, entries)
		// Recompute entry boundaries after sort (positions shifted).
		lineNums = computeLineNumbers(tokens)
		depths = computeDepths(tokens, lineNums, structuralLines)
		entries = groupTopLevelEntries(tokens, depths, from, len(tokens), targetDepth, lineNums, structuralLines)
	}

	// Recursively sort nested mappings within each entry.
	for _, e := range entries {
		tokens = sortKeysAtDepth(tokens, depths, targetDepth+1, e.startIdx, e.endIdx, lineNums, structuralLines)
		// Recompute after nested sort.
		lineNums = computeLineNumbers(tokens)
		depths = computeDepths(tokens, lineNums, structuralLines)
		_ = groupTopLevelEntries(tokens, depths, from, len(tokens), targetDepth, lineNums, structuralLines)
	}

	return tokens
}

// mappingEntry represents a key-value entry in a mapping, including
// leading comments and all nested content.
type mappingEntry struct {
	startIdx int    // index into token stream where this entry starts
	endIdx   int    // index past the last token of this entry
	key      string // decoded key for sort comparison
}

// groupTopLevelEntries finds all mapping entries at targetDepth within
// the token range [from, to). An entry starts at its key token and extends
// until the next key at the same depth within this range.
func groupTopLevelEntries(tokens []Token, depths []int, from, to, targetDepth int, lineNums []int, structuralLines map[int]bool) []mappingEntry {
	var entries []mappingEntry

	for i := from; i < to; i++ {
		tok := tokens[i]
		if tok.Kind != TokKey {
			continue
		}
		// Only consider keys on structural lines (skip continuation values
		// that happen to contain a colon).
		if structuralLines != nil && lineNums != nil && i < len(lineNums) {
			if !structuralLines[lineNums[i]] {
				continue
			}
		}
		keyDepth := findKeyDepth(tokens, depths, i)
		if keyDepth != targetDepth {
			continue
		}

		entry := mappingEntry{
			startIdx: findEntryStart(tokens, i),
			key:      string(tok.Raw),
		}
		entries = append(entries, entry)
	}

	// Set endIdx: each entry extends until the start of the next entry at this depth.
	for i := range entries {
		if i+1 < len(entries) {
			entries[i].endIdx = entries[i+1].startIdx
		} else {
			// Last entry: extend to end of range, but exclude trailing
			// whitespace-only tokens (indent/newline with no content after them).
			end := to
			for end > entries[i].startIdx {
				tok := tokens[end-1]
				if tok.Kind == TokIndent || tok.Kind == TokNewline || tok.Kind == TokSpace {
					end--
				} else {
					break
				}
			}
			// But keep the final newline if it follows content.
			if end < to && tokens[end].Kind == TokNewline {
				end++
			}
			entries[i].endIdx = end
		}
	}

	return entries
}

// sortEntrySlice sorts entries by key and reassembles the token stream.
func sortEntrySlice(tokens []Token, entries []mappingEntry) []Token {
	if len(entries) < 2 {
		return tokens
	}

	// Tokens before the first entry.
	result := make([]Token, 0, len(tokens))
	if entries[0].startIdx > 0 {
		result = append(result, tokens[:entries[0].startIdx]...)
	}

	// Sort entries by key.
	sorted := make([]mappingEntry, len(entries))
	copy(sorted, entries)
	sortByKey(sorted)

	// Emit sorted entries, ensuring newline separation.
	for i, e := range sorted {
		entryTokens := tokens[e.startIdx:e.endIdx]
		result = append(result, entryTokens...)

		// If this isn't the last entry and doesn't end with a newline, add one.
		if i < len(sorted)-1 && len(entryTokens) > 0 {
			last := entryTokens[len(entryTokens)-1]
			if last.Kind != TokNewline {
				result = append(result, Token{Kind: TokNewline, Raw: []byte("\n")})
			}
		}
	}

	// Tokens after the last entry.
	lastEnd := entries[len(entries)-1].endIdx
	if lastEnd < len(tokens) {
		result = append(result, tokens[lastEnd:]...)
	}

	return result
}

// findKeyDepth determines the depth of a key token by looking at the
// preceding indent token.
func findKeyDepth(tokens []Token, depths []int, keyIdx int) int {
	// Walk backwards to find the indent token for this line.
	for j := keyIdx - 1; j >= 0; j-- {
		if tokens[j].Kind == TokIndent {
			if depths[j] >= 0 {
				return depths[j]
			}
			return 0
		}
		if tokens[j].Kind == TokNewline {
			// No indent before this key on this line — it's at depth 0.
			return 0
		}
	}
	return 0
}

// findEntryStart finds the first token index that belongs to this entry,
// including the indent token, leading dash/tag/anchor, and leading comments.
func findEntryStart(tokens []Token, keyIdx int) int {
	start := keyIdx

	// Walk backwards past preceding tokens on the same line (dash, tag, anchor, space).
	for start > 0 {
		prev := tokens[start-1]
		if prev.Kind == TokDash || prev.Kind == TokTag || prev.Kind == TokAnchor || prev.Kind == TokSpace {
			start--
		} else if prev.Kind == TokIndent {
			start--
			break
		} else {
			break
		}
	}

	// Walk further back to include leading comments at the same indent level.
	// A leading comment is one that is the ONLY non-whitespace content on its line
	// (preceded by indent or start-of-line). Inline comments (after a key/value
	// on the same line) belong to the previous entry.
	for start > 0 {
		pos := start - 1
		// Skip newline before this line.
		if pos < 0 || tokens[pos].Kind != TokNewline {
			break
		}
		pos--
		// Look for comment.
		if pos < 0 || tokens[pos].Kind != TokComment {
			break
		}
		commentPos := pos
		pos--
		// Check what precedes the comment — must be indent or newline (start of line).
		// If it's anything else (colon, value, key, etc.) → inline comment, stop.
		if pos >= 0 && tokens[pos].Kind == TokIndent {
			start = pos
		} else if pos < 0 || tokens[pos].Kind == TokNewline {
			// Comment at column 0.
			start = commentPos
		} else {
			// Inline comment on previous entry's line — don't grab it.
			break
		}
	}

	return start
}

// sortByKey sorts entries alphabetically by key.
func sortByKey(entries []mappingEntry) {
	slices.SortStableFunc(entries, func(a, b mappingEntry) int {
		return strings.Compare(a.key, b.key)
	})
}

// stripTrailingWhitespace removes trailing spaces and tabs from each line.
func stripTrailingWhitespace(data []byte) []byte {
	var result []byte
	lineStart := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			// Trim trailing spaces/tabs from this line.
			end := i
			for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
				end--
			}
			result = append(result, data[lineStart:end]...)
			result = append(result, '\n')
			lineStart = i + 1
		} else if data[i] == '\r' {
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
		}
	}
	// Handle last line (no trailing newline).
	if lineStart < len(data) {
		end := len(data)
		for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
			end--
		}
		result = append(result, data[lineStart:end]...)
	}
	return result
}
