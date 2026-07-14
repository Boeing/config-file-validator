package yamlfmt

import (
	"bytes"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted takes a token stream and formatting options, applies
// indent normalization, optional key sorting, and quote style, then serializes.
func printFormatted(tokens []Token, opts formatter.Options, src []byte) []byte {
	if len(tokens) == 0 {
		return nil
	}

	targetWidth := opts.IndentWidth
	if targetWidth <= 0 {
		targetWidth = 2
	}

	// Annotate tokens with structural metadata from yaml.v3 Node tree.
	annotate(tokens, src)

	// Sort keys if requested. Metadata travels with tokens.
	if opts.SortKeys {
		tokens = sortKeys(tokens)
	}

	// Reindent: structural tokens get depth×width; continuations shift by parent delta.
	reindentTokens(tokens, targetWidth)

	// Apply quote style preference.
	if opts.QuoteStyle != formatter.QuotePreserve {
		applyQuoteStyle(tokens, opts.QuoteStyle)
	}

	// Normalize space before inline comments to exactly one space.
	for i := range tokens {
		if tokens[i].Kind == TokSpace && i+1 < len(tokens) && tokens[i+1].Kind == TokComment {
			if i > 0 && tokens[i-1].Kind != TokIndent && tokens[i-1].Kind != TokNewline {
				tokens[i].Raw = []byte(" ")
			}
		}
	}

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

// =============================================================================
// Annotation: set Structural and Depth on tokens using yaml.v3 Node tree
// =============================================================================

// annotate sets Structural and Depth on each TokIndent token.
// Uses the yaml.v3 Node tree to determine which lines are structural
// (mapping keys, sequence items) vs continuation (multi-line values).
func annotate(tokens []Token, src []byte) {
	// Build set of structural line numbers from Node tree.
	structuralLines := buildStructuralLineSet(src)

	// Compute line number for each token.
	line := 1
	for i := range tokens {
		tokens[i].Depth = -1

		if tokens[i].Kind == TokIndent {
			// Skip blank lines (indent followed by newline).
			if i+1 < len(tokens) && tokens[i+1].Kind == TokNewline {
				continue
			}
			// Comments are structural (they should be reindented with their context).
			if i+1 < len(tokens) && tokens[i+1].Kind == TokComment {
				tokens[i].Structural = true
			} else {
				tokens[i].Structural = structuralLines == nil || structuralLines[line]
			}
		}

		for _, b := range tokens[i].Raw {
			if b == '\n' {
				line++
			}
		}
	}

	// Compute depths using a stack — only structural indents participate.
	type level struct {
		indent int
		depth  int
	}
	stack := []level{{0, 0}}

	for i := range tokens {
		if tokens[i].Kind != TokIndent || !tokens[i].Structural {
			continue
		}
		indent := len(tokens[i].Raw)

		// Pop stack until parent.indent < indent.
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]

		if indent > parent.indent {
			newDepth := parent.depth + 1
			stack = append(stack, level{indent, newDepth})
			tokens[i].Depth = newDepth
		} else {
			tokens[i].Depth = parent.depth
		}
	}
}

// buildStructuralLineSet parses YAML and returns line numbers containing
// mapping keys or sequence items. Returns nil on parse failure (safe default:
// treat all lines as structural).
func buildStructuralLineSet(src []byte) map[int]bool {
	// Ensure trailing newline for consistent parsing.
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(bytes.Clone(src), '\n')
	}
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil
	}
	lines := make(map[int]bool)
	collectStructuralLines(&root, lines)
	return lines
}

func collectStructuralLines(n *yaml.Node, lines map[int]bool) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			collectStructuralLines(c, lines)
		}
	case yaml.MappingNode:
		lines[n.Line] = true
		for i := 0; i < len(n.Content); i += 2 {
			lines[n.Content[i].Line] = true
			if i+1 < len(n.Content) {
				collectStructuralLines(n.Content[i+1], lines)
			}
		}
	case yaml.SequenceNode:
		lines[n.Line] = true
		for _, item := range n.Content {
			lines[item.Line] = true
			collectStructuralLines(item, lines)
		}
	default:
		// Scalar, alias — no structural children.
	}
}

// =============================================================================
// Reindent
// =============================================================================

// reindentTokens modifies TokIndent.Raw based on Structural + Depth.
// Structural tokens: new indent = Depth × targetWidth.
// Continuation tokens: shift by same delta as last structural token.
// Block scalars following a shifted indent also get their content shifted.
func reindentTokens(tokens []Token, targetWidth int) {
	lastDelta := 0

	for i := range tokens {
		if tokens[i].Kind != TokIndent {
			continue
		}
		oldIndent := len(tokens[i].Raw)

		var newIndent int
		if tokens[i].Structural && tokens[i].Depth >= 0 {
			newIndent = tokens[i].Depth * targetWidth
			lastDelta = newIndent - oldIndent
		} else {
			newIndent = oldIndent + lastDelta
			if newIndent < 0 {
				newIndent = 0
			}
		}

		tokens[i].Raw = []byte(strings.Repeat(" ", newIndent))
		delta := newIndent - oldIndent

		// Shift block scalar content on this line by same delta.
		if delta != 0 {
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].Kind == TokNewline {
					break
				}
				if tokens[j].Kind == TokBlockScalar {
					tokens[j].Raw = shiftBlockScalarIndent(tokens[j].Raw, delta)
					break
				}
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
		result = append(result, []byte(strings.Repeat(" ", newSpaces))...)

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

// =============================================================================
// Sort keys
// =============================================================================

type mappingEntry struct {
	startIdx int
	endIdx   int
	key      string
}

// sortKeys sorts mapping entries at all depth levels.
func sortKeys(tokens []Token) []Token {
	return sortKeysAtDepth(tokens, 0, 0, len(tokens))
}

func sortKeysAtDepth(tokens []Token, targetDepth, from, to int) []Token {
	entries := groupEntries(tokens, from, to, targetDepth)

	if len(entries) >= 2 {
		tokens = reorderEntries(tokens, entries)
		// Recompute entries after reorder.
		entries = groupEntries(tokens, 0, len(tokens), targetDepth)
	}

	// Recurse into nested mappings.
	for _, e := range entries {
		tokens = sortKeysAtDepth(tokens, targetDepth+1, e.startIdx, e.endIdx)
	}

	return tokens
}

// groupEntries finds mapping entries at the target depth within [from, to).
// Only considers keys on structural lines.
func groupEntries(tokens []Token, from, to, targetDepth int) []mappingEntry {
	var entries []mappingEntry

	for i := from; i < to; i++ {
		if tokens[i].Kind != TokKey {
			continue
		}
		// The key must be on a structural line — check the preceding indent.
		indentIdx := findPrecedingIndent(tokens, i)
		if indentIdx >= 0 && !tokens[indentIdx].Structural {
			continue
		}
		// Check depth matches target.
		depth := 0
		if indentIdx >= 0 {
			depth = tokens[indentIdx].Depth
		}
		if depth != targetDepth {
			continue
		}

		entries = append(entries, mappingEntry{
			startIdx: findEntryStart(tokens, i),
			key:      string(tokens[i].Raw),
		})
	}

	// Set endIdx.
	for i := range entries {
		if i+1 < len(entries) {
			entries[i].endIdx = entries[i+1].startIdx
		} else {
			// Last entry: exclude trailing whitespace.
			end := to
			for end > entries[i].startIdx {
				tok := tokens[end-1]
				if tok.Kind != TokIndent && tok.Kind != TokNewline && tok.Kind != TokSpace {
					break
				}
				end--
			}
			if end < to && tokens[end].Kind == TokNewline {
				end++
			}
			entries[i].endIdx = end
		}
	}

	return entries
}

// findPrecedingIndent finds the TokIndent before a key on the same line.
func findPrecedingIndent(tokens []Token, keyIdx int) int {
	for j := keyIdx - 1; j >= 0; j-- {
		switch tokens[j].Kind {
		case TokIndent:
			return j
		case TokDash, TokTag, TokAnchor, TokSpace:
			continue
		default:
			return -1 // newline or other content — no indent on this line
		}
	}
	return -1
}

// findEntryStart walks back to include indent + leading comments.
func findEntryStart(tokens []Token, keyIdx int) int {
	start := keyIdx

	// Walk back past same-line prefix tokens.
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

	// Walk back to include leading comments (standalone comment lines only).
	for start > 0 {
		pos := start - 1
		if pos < 0 || tokens[pos].Kind != TokNewline {
			break
		}
		pos--
		if pos < 0 || tokens[pos].Kind != TokComment {
			break
		}
		commentPos := pos
		pos--
		if pos >= 0 && tokens[pos].Kind == TokIndent {
			start = pos
		} else if pos < 0 || tokens[pos].Kind == TokNewline {
			start = commentPos
		} else {
			// Inline comment — don't grab.
			break
		}
	}

	return start
}

// reorderEntries sorts entries by key and reassembles the token stream.
func reorderEntries(tokens []Token, entries []mappingEntry) []Token {
	if len(entries) < 2 {
		return tokens
	}

	result := make([]Token, 0, len(tokens))
	if entries[0].startIdx > 0 {
		result = append(result, tokens[:entries[0].startIdx]...)
	}

	sorted := make([]mappingEntry, len(entries))
	copy(sorted, entries)
	slices.SortStableFunc(sorted, func(a, b mappingEntry) int {
		return strings.Compare(a.key, b.key)
	})

	for i, e := range sorted {
		entryTokens := tokens[e.startIdx:e.endIdx]
		result = append(result, entryTokens...)
		// Ensure newline separation between entries.
		if i < len(sorted)-1 && len(entryTokens) > 0 {
			if entryTokens[len(entryTokens)-1].Kind != TokNewline {
				result = append(result, Token{Kind: TokNewline, Raw: []byte("\n")})
			}
		}
	}

	lastEnd := entries[len(entries)-1].endIdx
	if lastEnd < len(tokens) {
		result = append(result, tokens[lastEnd:]...)
	}

	return result
}

// =============================================================================
// Quote style
// =============================================================================

func applyQuoteStyle(tokens []Token, style formatter.QuoteStyle) {
	for i := range tokens {
		if tokens[i].Kind != TokValue {
			continue
		}
		raw := tokens[i].Raw
		if len(raw) < 2 {
			continue
		}
		first, last := raw[0], raw[len(raw)-1]
		if first == '"' && last == '"' {
			tokens[i].Raw = convertQuote(raw, style, '"')
		} else if first == '\'' && last == '\'' {
			tokens[i].Raw = convertQuote(raw, style, '\'')
		}
	}
}

func convertQuote(raw []byte, style formatter.QuoteStyle, currentQuote byte) []byte {
	content := raw[1 : len(raw)-1]

	// Don't convert multi-line scalars.
	for _, b := range content {
		if b == '\n' || b == '\r' {
			return raw
		}
	}

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
