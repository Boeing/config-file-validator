package yamlfmt

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted takes a token stream and formatting options, applies
// indent normalization and optional key sorting, then serializes.
func printFormatted(tokens []Token, opts formatter.Options) []byte {
	if len(tokens) == 0 {
		return nil
	}

	targetWidth := opts.IndentWidth
	if targetWidth <= 0 {
		targetWidth = 2
	}

	// Compute depth for each IndentToken.
	depths := computeDepths(tokens)

	// Sort keys if requested.
	if opts.SortKeys {
		tokens = sortKeys(tokens, depths)
		// Recompute depths after sorting (positions may have shifted).
		depths = computeDepths(tokens)
	}

	// Reindent: replace each IndentToken with normalized indent.
	tokens = applyIndent(tokens, depths, targetWidth)

	// Serialize: concatenate all Raw fields.
	var buf bytes.Buffer
	for _, tok := range tokens {
		buf.Write(tok.Raw)
	}

	out := buf.Bytes()

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
func computeDepths(tokens []Token) []int {
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

		indent := len(tok.Raw)

		// Find where this indent fits in the stack.
		// Pop until we find an ancestor with indent < current.
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		if indent == parent.indent {
			// Same level as parent — same depth.
			depths[i] = parent.depth
		} else if indent > parent.indent {
			// Deeper — new child level.
			newDepth := parent.depth + 1
			stack = append(stack, level{indent, newDepth})
			depths[i] = newDepth
		} else {
			// This shouldn't happen after the pop loop, but handle gracefully.
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
func applyIndent(tokens []Token, depths []int, targetWidth int) []Token {
	result := make([]Token, len(tokens))
	copy(result, tokens)

	for i, tok := range result {
		if tok.Kind != TokIndent || depths[i] < 0 {
			continue
		}
		newIndent := depths[i] * targetWidth
		result[i] = Token{
			Kind: TokIndent,
			Raw:  []byte(strings.Repeat(" ", newIndent)),
		}
	}

	return result
}

// sortKeys sorts mapping entries within each scope by their key.
// An entry is: optional leading comments + key + colon + value/nested content.
// Entries are siblings if they share the same parent indent level.
func sortKeys(tokens []Token, depths []int) []Token {
	// Find mapping scopes and sort within each.
	entries := groupTopLevelEntries(tokens, depths, 0, len(tokens), 0)
	if len(entries) < 2 {
		return tokens
	}
	return sortEntrySlice(tokens, entries)
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
func groupTopLevelEntries(tokens []Token, depths []int, from, to, targetDepth int) []mappingEntry {
	var entries []mappingEntry

	for i := from; i < to; i++ {
		tok := tokens[i]
		if tok.Kind != TokKey {
			continue
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
			entries[i].endIdx = to
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

	// Emit sorted entries.
	for _, e := range sorted {
		result = append(result, tokens[e.startIdx:e.endIdx]...)
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
// including the indent token and leading comments.
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

	return start
}

// sortByKey sorts entries alphabetically by key.
func sortByKey(entries []mappingEntry) {
	slices.SortStableFunc(entries, func(a, b mappingEntry) int {
		return strings.Compare(a.key, b.key)
	})
}
