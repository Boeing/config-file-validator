// Sort keys phase: reorders mapping entries alphabetically at each depth level.
package yamlfmt

import (
	"slices"
	"strings"
)

type mappingEntry struct {
	startIdx int
	endIdx   int
	key      string
}

// sortKeys sorts mapping entries at all depth levels.
func sortKeys(at AnnotatedTokens) AnnotatedTokens {
	return AnnotatedTokens{tokens: sortKeysAtDepth(at.Tokens(), 0, 0, len(at.Tokens()))}
}

func sortKeysAtDepth(tokens []Token, targetDepth, from, to int) []Token {
	// Find sequence item boundaries within [from, to). Keys in different
	// sequence items are in different mappings and must not be sorted together.
	subRanges := splitBySeqItems(tokens, from, to, targetDepth)

	for _, sr := range subRanges {
		entries := groupEntries(tokens, sr.from, sr.to, targetDepth)

		if len(entries) >= 2 && !hasAnchorAliasDependency(tokens, entries) {
			tokens = reorderEntries(tokens, entries)
			// ASTDepth is position-invariant — no recomputation needed.
			entries = groupEntries(tokens, sr.from, sr.to, targetDepth)
		}

		// Recurse into nested mappings.
		for _, e := range entries {
			tokens = sortKeysAtDepth(tokens, targetDepth+1, e.startIdx, e.endIdx)
		}
	}

	return tokens
}

// subRange represents a contiguous range of tokens that belong to a single mapping scope.
type subRange struct {
	from, to int
}

// splitBySeqItems splits [from, to) into sub-ranges by detecting TokDash tokens
// that indicate sequence item boundaries at or below targetDepth.
// If no dashes are found, the entire range is returned as one sub-range.
func splitBySeqItems(tokens []Token, from, to, targetDepth int) []subRange {
	// Find all dash positions that start sequence items at targetDepth.
	// A dash at targetDepth means a new mapping scope for keys at targetDepth.
	var dashPositions []int
	for i := from; i < to; i++ {
		if tokens[i].Kind != TokDash {
			continue
		}
		// Check the indent token preceding this dash — its ASTDepth tells us
		// the depth of this sequence item.
		indentIdx := findPrecedingIndent(tokens, i)
		if indentIdx >= 0 && tokens[indentIdx].ASTDepth == targetDepth && tokens[indentIdx].InSeq {
			dashPositions = append(dashPositions, indentIdx)
		}
	}

	if len(dashPositions) == 0 {
		return []subRange{{from: from, to: to}}
	}

	var ranges []subRange
	// Before the first dash (if there are keys before it at this depth).
	if dashPositions[0] > from {
		ranges = append(ranges, subRange{from: from, to: dashPositions[0]})
	}
	// Each dash starts a new sub-range.
	for i, dp := range dashPositions {
		end := to
		if i+1 < len(dashPositions) {
			end = dashPositions[i+1]
		}
		ranges = append(ranges, subRange{from: dp, to: end})
	}
	return ranges
}

// hasAnchorAliasDependency checks whether reordering entries would break
// anchor/alias references. Returns true if any alias in one entry references
// an anchor defined in a different entry within the same scope.
//
// When this returns true, the caller skips sorting for this scope to avoid
// producing invalid YAML where an alias appears before its anchor definition.
// Nested mappings within each entry are still sorted independently.
func hasAnchorAliasDependency(tokens []Token, entries []mappingEntry) bool {
	// Map anchor names to the entry index that defines them.
	anchorOwner := make(map[string]int)
	for i, e := range entries {
		for j := e.startIdx; j < e.endIdx; j++ {
			if tokens[j].Kind == TokAnchor {
				// Anchor raw is "&name" — strip the leading &.
				name := strings.TrimPrefix(string(tokens[j].Raw), "&")
				anchorOwner[name] = i
			}
		}
	}

	// If no anchors exist in this scope, sorting is safe.
	if len(anchorOwner) == 0 {
		return false
	}

	// Check if any alias references an anchor from a DIFFERENT entry.
	for i, e := range entries {
		for j := e.startIdx; j < e.endIdx; j++ {
			if tokens[j].Kind == TokAlias {
				// Alias raw is "*name" — strip the leading *.
				name := strings.TrimPrefix(string(tokens[j].Raw), "*")
				if ownerIdx, exists := anchorOwner[name]; exists && ownerIdx != i {
					return true
				}
			}
		}
	}

	return false
}

// groupEntries finds mapping entries at the target depth within [from, to).
// Uses AST-derived depth for grouping.
func groupEntries(tokens []Token, from, to, targetDepth int) []mappingEntry {
	var entries []mappingEntry

	for i := from; i < to; i++ {
		if tokens[i].Kind != TokKey {
			continue
		}
		// Use AST-derived depth for grouping.
		if tokens[i].ASTDepth != targetDepth {
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
