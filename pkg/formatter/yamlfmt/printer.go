package yamlfmt

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted takes a token stream and formatting options, applies
// indent normalization, optional key sorting, and quote style, then serializes.
func printFormatted(tokens []Token, opts formatter.Options, src []byte) ([]byte, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	targetWidth := opts.IndentWidth
	if targetWidth <= 0 {
		targetWidth = 2
	}

	// Build AST metadata for structure-aware formatting.
	astMeta, err := buildASTMetadata(src)
	if err != nil {
		return nil, err
	}
	flowNodes := buildFlowNodeMap(src)

	// Annotate tokens with structural metadata from yaml.v3 Node tree.
	annotate(tokens, src, astMeta)

	// Normalize flow collections using AST-driven re-serialization.
	normalizeFlowTokens(tokens, flowNodes)

	// Normalize value spacing: strip leading whitespace from values after colons.
	normalizeValueSpacing(tokens)

	// Sort keys if requested. Metadata travels with tokens.
	if opts.SortKeys {
		tokens = sortKeys(tokens)
		// No depth recomputation needed — ASTDepth is position-invariant.
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

	// Serialize, stripping trailing whitespace from non-block-scalar lines.
	out := serializeWithStrip(tokens)

	// Trim trailing newlines — but preserve them for |+ (keep chomping).
	if !endsWithKeepChomping(tokens) {
		out = bytes.TrimRight(out, "\r\n")
	}
	if opts.FinalNewline && (len(out) == 0 || out[len(out)-1] != '\n') {
		out = append(out, '\n')
	}

	return formatter.NormalizeLineEndings(out, opts.LineEnding), nil
}

// =============================================================================
// Annotation: set Structural, Line, ASTDepth, InSeq on tokens using yaml.v3 Node tree
// =============================================================================

type lineMetadata struct {
	depth     int
	inSeq     bool
	seqOffset int // number of ancestor non-dash sequence levels contributing +2 each
}

// buildASTMetadata walks the yaml.v3 Node tree and returns metadata for each
// line that contains a mapping key: its semantic depth and whether it's inside
// a sequence item.
func buildASTMetadata(src []byte) (map[int]lineMetadata, error) {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(bytes.Clone(src), '\n')
	}
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("cannot determine document structure: %w", err)
	}
	meta := make(map[int]lineMetadata)
	collectMetadata(&root, meta, 0, false, 0)
	return meta, nil
}

//nolint:revive // inSeq is a recursive state parameter, not a control flag
func collectMetadata(n *yaml.Node, meta map[int]lineMetadata, depth int, inSeq bool, seqOffset int) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			collectMetadata(c, meta, depth, false, seqOffset)
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			// Only write if no shallower entry exists at this line.
			// This prevents flow values ({a: 1}) on the same line as
			// their parent key from overwriting the parent's metadata.
			if existing, ok := meta[key.Line]; !ok || existing.depth > depth {
				meta[key.Line] = lineMetadata{depth: depth, inSeq: inSeq, seqOffset: seqOffset}
			}
			if i+1 < len(n.Content) {
				// Children of a non-dash seq key inherit seqOffset+1 because
				// the parent is inSeq without a dash (it contributes +2).
				childOffset := seqOffset
				if inSeq {
					childOffset = seqOffset + 1
				}
				collectMetadata(n.Content[i+1], meta, depth+1, false, childOffset)
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			// Record the start line of each sequence item at this depth.
			if item.Line > 0 {
				if existing, ok := meta[item.Line]; !ok || existing.depth > depth {
					meta[item.Line] = lineMetadata{depth: depth, inSeq: true, seqOffset: seqOffset}
				}
			}
			if item.Kind == yaml.SequenceNode {
				// Nested sequence: inner items are one level deeper.
				// Don't increment seqOffset — the dash handles positioning.
				collectMetadata(item, meta, depth+1, true, seqOffset)
			} else {
				collectMetadata(item, meta, depth, true, seqOffset)
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// Bare sequence items (scalars/aliases not inside a mapping key/value
		// pair) need metadata so their lines get proper ASTDepth.
		if inSeq && n.Line > 0 {
			if existing, ok := meta[n.Line]; !ok || existing.depth > depth {
				meta[n.Line] = lineMetadata{depth: depth, inSeq: true, seqOffset: seqOffset}
			}
		}
	default:
		// Unknown node kind — no metadata to collect.
	}
}

// assignASTMetadata sets ASTDepth and InSeq on tokens by matching each TokKey
// or TokDash to the AST metadata for its line. Also propagates to the preceding
// TokIndent and to standalone comment lines that precede structural lines.
func assignASTMetadata(tokens []Token, meta map[int]lineMetadata) {
	// Pass 1: Assign metadata from TokKey tokens.
	for i := range tokens {
		if tokens[i].Kind != TokKey {
			continue
		}
		lm, ok := meta[tokens[i].Line]
		if !ok {
			continue
		}
		tokens[i].ASTDepth = lm.depth
		tokens[i].InSeq = lm.inSeq
		tokens[i].SeqOffset = lm.seqOffset
		// Propagate to preceding indent token (reindent operates on indent tokens).
		indentIdx := findPrecedingIndent(tokens, i)
		if indentIdx >= 0 {
			tokens[indentIdx].ASTDepth = lm.depth
			tokens[indentIdx].InSeq = lm.inSeq
			tokens[indentIdx].SeqOffset = lm.seqOffset
		}
	}

	// Pass 2: Assign metadata from TokDash tokens on lines with no TokKey
	// (bare sequence items like "- alpha").
	for i := range tokens {
		if tokens[i].Kind != TokDash {
			continue
		}
		lm, ok := meta[tokens[i].Line]
		if !ok {
			continue
		}
		// Only assign if the preceding indent doesn't already have metadata
		// (a TokKey on the same line would have already set it in pass 1).
		indentIdx := findPrecedingIndent(tokens, i)
		if indentIdx >= 0 && tokens[indentIdx].ASTDepth < 0 {
			tokens[indentIdx].ASTDepth = lm.depth
			tokens[indentIdx].InSeq = lm.inSeq
			tokens[indentIdx].SeqOffset = lm.seqOffset
		}
	}

	// Pass 3: Propagate metadata to standalone comment lines.
	// A comment gets the same ASTDepth/InSeq/SeqOffset as the next structural indent.
	for i := range tokens {
		if tokens[i].Kind != TokIndent || !tokens[i].Structural || tokens[i].ASTDepth >= 0 {
			continue
		}
		// Check if this indent precedes a comment.
		if i+1 >= len(tokens) || tokens[i+1].Kind != TokComment {
			continue
		}
		// Find the next structural indent with assigned ASTDepth.
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Kind == TokIndent && tokens[j].Structural && tokens[j].ASTDepth >= 0 {
				tokens[i].ASTDepth = tokens[j].ASTDepth
				tokens[i].InSeq = tokens[j].InSeq
				tokens[i].SeqOffset = tokens[j].SeqOffset
				break
			}
		}
	}
}

// computeNewIndent returns the target indentation for a structural line.
//
//nolint:revive // inSeq/hasDash are structural properties, not control flags
func computeNewIndent(astDepth int, inSeq, hasDash bool, seqOffset, targetWidth int) int {
	base := astDepth*targetWidth + seqOffset*2
	if inSeq && !hasDash {
		return base + 2
	}
	return base
}

// annotate sets Structural, Line, ASTDepth, and InSeq on each token.
// Uses the yaml.v3 Node tree to determine which lines are structural
// (mapping keys, sequence items) vs continuation (multi-line values).
func annotate(tokens []Token, src []byte, astMeta map[int]lineMetadata) {
	// Build set of structural line numbers from Node tree.
	structuralLines := buildStructuralLineSet(src)

	// Compute line number for each token and set Structural flag.
	line := 1
	for i := range tokens {
		tokens[i].ASTDepth = -1
		tokens[i].Line = line

		if tokens[i].Kind == TokIndent {
			// Skip blank lines (indent followed by newline).
			if i+1 < len(tokens) && tokens[i+1].Kind == TokNewline {
				for _, b := range tokens[i].Raw {
					if b == '\n' {
						line++
					}
				}
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

	// Set ASTDepth and InSeq from AST metadata.
	assignASTMetadata(tokens, astMeta)
}

// lineHasDash checks whether a TokDash follows the indent at index i on the same line.
func lineHasDash(tokens []Token, indentIdx int) bool {
	for j := indentIdx + 1; j < len(tokens); j++ {
		switch tokens[j].Kind {
		case TokNewline, TokBlockScalar:
			return false
		case TokDash:
			return true
		default:
		}
	}
	return false
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

// reindentTokens modifies TokIndent.Raw based on Structural + ASTDepth.
// Structural tokens with ASTDepth >= 0: new indent = computeNewIndent(...).
// Continuation tokens (ASTDepth < 0 or non-structural): shift by same delta as
// last structural token. Block scalars following a shifted indent also get
// their content shifted.
func reindentTokens(tokens []Token, targetWidth int) {
	lastDelta := 0

	for i := range tokens {
		if tokens[i].Kind != TokIndent {
			continue
		}
		oldIndent := len(tokens[i].Raw)

		var newIndent int
		if tokens[i].Structural && tokens[i].ASTDepth >= 0 {
			hasDash := lineHasDash(tokens, i)
			newIndent = computeNewIndent(tokens[i].ASTDepth, tokens[i].InSeq, hasDash, tokens[i].SeqOffset, targetWidth)
			lastDelta = newIndent - oldIndent
		} else {
			newIndent = oldIndent + lastDelta
			if newIndent < 0 {
				newIndent = 0
			}
		}

		tokens[i].Raw = []byte(strings.Repeat(" ", newIndent))
		delta := newIndent - oldIndent

		// Shift block scalar content by same delta.
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
		// Trim trailing horizontal whitespace for quote boundary detection.
		// serializeWithStrip removes this from the final output anyway —
		// trimming here ensures the quote decision is idempotent regardless of
		// whether the input had trailing spaces/tabs after a quoted value.
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
	}
}

// isSimpleQuoted verifies that a raw value token is a straightforwardly quoted
// scalar — the first and last bytes are the matching quotes with actual content
// between them. Rejects edge cases like `”'` (escaped quote at boundary) or
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
// Value spacing normalization
// =============================================================================

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

// =============================================================================
// Flow collection normalization
// =============================================================================

// flowNodeMap maps [line, column] to the yaml.v3 Node for a flow collection.
type flowNodeMap map[[2]int]*yaml.Node

// buildFlowNodeMap parses src and returns a map of all flow-style collection
// nodes keyed by their [line, column] position.
func buildFlowNodeMap(src []byte) flowNodeMap {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(bytes.Clone(src), '\n')
	}
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil
	}
	m := make(flowNodeMap)
	collectFlowNodes(&root, m)
	return m
}

func collectFlowNodes(n *yaml.Node, m flowNodeMap) {
	if (n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode) && n.Style == yaml.FlowStyle {
		m[[2]int{n.Line, n.Column}] = n
	}
	for _, c := range n.Content {
		collectFlowNodes(c, m)
	}
}

// serializeFlowNode re-serializes a flow collection node with normalized spacing.
func serializeFlowNode(n *yaml.Node) []byte {
	var buf bytes.Buffer
	writeFlowNode(&buf, n)
	return buf.Bytes()
}

func writeFlowNode(buf *bytes.Buffer, n *yaml.Node) {
	switch n.Kind {
	case yaml.MappingNode:
		buf.WriteByte('{')
		for i := 0; i < len(n.Content); i += 2 {
			if i > 0 {
				buf.WriteString(", ")
			}
			writeFlowNode(buf, n.Content[i]) // key
			buf.WriteString(": ")
			writeFlowNode(buf, n.Content[i+1]) // value
		}
		buf.WriteByte('}')
	case yaml.SequenceNode:
		buf.WriteByte('[')
		for i, item := range n.Content {
			if i > 0 {
				buf.WriteString(", ")
			}
			writeFlowNode(buf, item)
		}
		buf.WriteByte(']')
	case yaml.ScalarNode:
		writeFlowScalar(buf, n)
	case yaml.AliasNode:
		buf.WriteByte('*')
		buf.WriteString(n.Value)
	default:
		// Unknown node kind — skip
	}
}

func writeFlowScalar(buf *bytes.Buffer, n *yaml.Node) {
	if n.Anchor != "" {
		buf.WriteByte('&')
		buf.WriteString(n.Anchor)
		buf.WriteByte(' ')
	}
	if n.Tag == "!!null" || (n.Tag == "" && n.Value == "") {
		buf.WriteString("null")
		return
	}
	switch n.Style {
	case yaml.DoubleQuotedStyle:
		buf.WriteByte('"')
		buf.WriteString(escapeDoubleQuoted(n.Value))
		buf.WriteByte('"')
	case yaml.SingleQuotedStyle:
		buf.WriteByte('\'')
		buf.WriteString(escapeSingleQuoted(n.Value))
		buf.WriteByte('\'')
	default:
		// Plain scalar. Quote if needed in flow context.
		if needsQuotingInFlow(n.Value, n.Tag) {
			buf.WriteByte('"')
			buf.WriteString(escapeDoubleQuoted(n.Value))
			buf.WriteByte('"')
		} else {
			buf.WriteString(n.Value)
		}
	}
}

func escapeDoubleQuoted(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeSingleQuoted(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func needsQuotingInFlow(value, tag string) bool {
	if value == "" {
		return true
	}
	// Characters that are flow indicators or ambiguous.
	for _, r := range value {
		switch r {
		case '{', '}', '[', ']', ',', ':', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
			return true
		}
	}
	// Values that look like other YAML types need quoting to preserve string semantics.
	if tag == "!!str" {
		switch strings.ToLower(value) {
		case "true", "false", "null", "~", "yes", "no", "on", "off":
			return true
		}
		if looksNumeric(value) {
			return true
		}
	}
	return false
}

func looksNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != 'e' && r != 'E' && r != '-' && r != '+' && r != '_' {
			return false
		}
	}
	return true
}

// flowHasComments returns true if the node or any of its children have comments.
// Flows with comments are not re-serialized to preserve the original formatting.
func flowHasComments(n *yaml.Node) bool {
	if n.HeadComment != "" || n.LineComment != "" || n.FootComment != "" {
		return true
	}
	for _, c := range n.Content {
		if flowHasComments(c) {
			return true
		}
	}
	return false
}

// normalizeFlowTokens replaces each TokFlow token with AST-driven re-serialized
// content for normalized spacing. Skips flows with comments.
func normalizeFlowTokens(tokens []Token, flowNodes flowNodeMap) {
	if flowNodes == nil {
		return
	}
	for i := range tokens {
		if tokens[i].Kind != TokFlow {
			continue
		}
		col := computeTokenColumn(tokens, i)
		node, ok := flowNodes[[2]int{tokens[i].Line, col}]
		if !ok {
			continue
		}
		if flowHasComments(node) {
			continue // preserve original for flows with comments
		}
		tokens[i].Raw = serializeFlowNode(node)
	}
}

// computeTokenColumn computes the 1-based column of a token by summing
// the lengths of tokens on the same line before it.
func computeTokenColumn(tokens []Token, idx int) int {
	col := 1
	for j := idx - 1; j >= 0; j-- {
		if tokens[j].Kind == TokNewline {
			break
		}
		col += len(tokens[j].Raw)
	}
	return col
}

// =============================================================================
// Utilities
// =============================================================================

// serializeWithStrip walks tokens, strips trailing whitespace from each line,
// but emits TokBlockScalar tokens verbatim to preserve block scalar semantics
// (trailing spaces in content lines and trailing newlines for |+ chomping).
func serializeWithStrip(tokens []Token) []byte {
	var out []byte
	var line []byte

	// flushLineStripped trims trailing spaces/tabs and appends to out.
	flushLineStripped := func() {
		end := len(line)
		for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t') {
			end--
		}
		out = append(out, line[:end]...)
		line = line[:0]
	}

	// flushLineRaw appends accumulated line content to out without stripping.
	flushLineRaw := func() {
		out = append(out, line...)
		line = line[:0]
	}

	for _, tok := range tokens {
		if tok.Kind == TokBlockScalar {
			// Flush pending line content WITHOUT stripping — the trailing
			// space (e.g. from ": ") is needed before the block scalar header.
			flushLineRaw()
			// Emit block scalar raw, verbatim — no stripping.
			out = append(out, tok.Raw...)
			continue
		}

		// Accumulate into line buffer; flush on newlines.
		for _, b := range tok.Raw {
			if b == '\n' {
				// Check for CRLF: if line ends with \r, include it in the line
				// before stripping (strip only spaces/tabs).
				hasCR := len(line) > 0 && line[len(line)-1] == '\r'
				if hasCR {
					line = line[:len(line)-1]
				}
				flushLineStripped()
				if hasCR {
					out = append(out, '\r')
				}
				out = append(out, '\n')
			} else {
				line = append(line, b)
			}
		}
	}

	// Flush remaining content (strip trailing whitespace).
	flushLineStripped()
	return out
}

// endsWithKeepChomping checks whether the last meaningful token in the stream
// is a block scalar with keep (+) chomping. When true, the trailing newlines
// after the scalar are semantically significant and must not be trimmed.
func endsWithKeepChomping(tokens []Token) bool {
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i].Kind {
		case TokNewline, TokIndent, TokSpace:
			continue
		case TokBlockScalar:
			return blockScalarHasKeepChomping(tokens[i].Raw)
		default:
			return false
		}
	}
	return false
}

// blockScalarHasKeepChomping checks if a block scalar's header contains '+'.
func blockScalarHasKeepChomping(raw []byte) bool {
	// Header is everything before the first newline.
	nlIdx := bytes.IndexByte(raw, '\n')
	if nlIdx < 0 {
		return false // malformed — no newline in block scalar
	}
	return bytes.IndexByte(raw[:nlIdx], '+') >= 0
}
