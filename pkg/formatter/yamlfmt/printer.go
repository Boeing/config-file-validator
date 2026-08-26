package yamlfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	astMeta, err := buildASTMetadata(src, tokens)
	if err != nil {
		return nil, err
	}

	// Annotate tokens with structural metadata from yaml.v3 Node tree.
	annotate(tokens, src, astMeta)

	// Match Prettier's bracketSpacing behavior for flow mappings without
	// rewriting the user's colon or comma spacing.
	normalizeFlowTokens(tokens)

	// Normalize value spacing: strip leading whitespace from values after colons.
	normalizeValueSpacing(tokens)

	// Sort keys if requested. Metadata travels with tokens.
	if opts.SortKeys {
		tokens = sortKeys(tokens)
		// No depth recomputation needed — ASTDepth is position-invariant.
	}

	// Reindent: structural tokens get depth×width; continuations shift by parent delta.
	reindentTokens(tokens, targetWidth,
		opts.IndentSequences != formatter.SequenceIndentDisabled)

	// Trim trailing blank lines from clip-chomped block scalars.
	trimBlockScalarTrailingBlanks(tokens)

	// Apply quote style preference.
	if opts.QuoteStyle != formatter.QuotePreserve {
		applyQuoteStyle(tokens, opts.QuoteStyle)
	}

	// Normalize space before inline comments to exactly one space.
	// When preceded by TokColon (which already includes a trailing space),
	// the extra space is removed entirely to avoid double-spacing.
	for i := range tokens {
		if tokens[i].Kind == TokSpace && i+1 < len(tokens) && tokens[i+1].Kind == TokComment {
			if i > 0 && tokens[i-1].Kind != TokIndent && tokens[i-1].Kind != TokNewline {
				if tokens[i-1].Kind == TokColon {
					// TokColon already has a trailing space — remove the extra.
					tokens[i].Raw = nil
				} else {
					tokens[i].Raw = []byte(" ")
				}
			}
		}
	}

	normalizeSequenceSpacing(tokens)

	// Expand flow sequences that exceed print width.
	// Per prettier: flow sequences that exceed printWidth get expanded to
	// multi-line flow format with each element on its own line.
	// Source: src/language-yaml/print/flow-mapping-sequence.js — group breaks at printWidth
	if opts.MaxLineWidth > 0 {
		expandLongFlowSequences(tokens, opts.MaxLineWidth, opts.IndentWidth)
	}

	// Collapse consecutive blank lines to at most 1 (matches prettier).
	tokens = collapseConsecutiveBlankLines(tokens)

	// Strip blank lines immediately after document markers (--- and ...).
	// prettier: join(hardline, parts) — exactly one newline after marker, never two.
	tokens = stripBlankLinesAfterDocMarkers(tokens)

	// Strip blank lines between a mapping key's colon and its first child value.
	// Prettier rule: no blank line allowed between key: and its value.
	// Only between siblings (between sequence items, between mapping entries).
	tokens = stripBlankLinesAfterColon(tokens)

	// Serialize, stripping trailing whitespace from non-block-scalar lines.
	out := serializeWithStrip(tokens)

	// Trim trailing newlines — but preserve them for |+ (keep chomping).
	if !endsWithBlockScalarPreservingNewlines(tokens) {
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
	depth               int
	inSeq               bool
	seqOffset           int // number of ancestor non-dash sequence levels contributing +2 each
	sequenceIndentDepth int // mapping-value sequence levels affecting indentation
}

// buildASTMetadata walks the yaml.v3 Node tree and returns metadata for each
// line that contains a mapping key: its semantic depth and whether it's inside
// a sequence item.
func buildASTMetadata(src []byte, tokens []Token) (map[int]lineMetadata, error) {
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(bytes.Clone(src), '\n')
	}
	meta := make(map[int]lineMetadata)
	dashCols := sequenceDashColumns(tokens)
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var root yaml.Node
		if err := dec.Decode(&root); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("cannot determine document structure: %w", err)
		}
		collectMetadata(&root, meta, dashCols, 0, false, 0, 0)
	}
	return meta, nil
}

//nolint:revive // inSeq is a recursive state parameter, not a control flag
func collectMetadata(n *yaml.Node, meta map[int]lineMetadata, dashColumns map[int][]int,
	depth int, inSeq bool, seqOffset, sequenceIndentDepth int) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			collectMetadata(c, meta, dashColumns, depth, false, seqOffset, sequenceIndentDepth)
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			// Only write if no shallower entry exists at this line.
			// This prevents flow values ({a: 1}) on the same line as
			// their parent key from overwriting the parent's metadata.
			if existing, ok := meta[key.Line]; !ok || existing.depth > depth {
				meta[key.Line] = lineMetadata{
					depth: depth, inSeq: inSeq, seqOffset: seqOffset,
					sequenceIndentDepth: sequenceIndentDepth,
				}
			}
			if i+1 < len(n.Content) {
				// Children of a non-dash seq key inherit seqOffset+1 because
				// the parent is inSeq without a dash (it contributes +2).
				childOffset := seqOffset
				if inSeq {
					childOffset = seqOffset + 1
				}
				childSequenceIndentDepth := sequenceIndentDepth
				if n.Content[i+1].Kind == yaml.SequenceNode {
					childSequenceIndentDepth++
				}
				collectMetadata(n.Content[i+1], meta, dashColumns, depth+1, false,
					childOffset, childSequenceIndentDepth)
			}
		}
	case yaml.SequenceNode:
		// A sequence node starts on its first dash. Recording the node itself
		// preserves the outer dash when a dash-only item contains another
		// sequence on the following line.
		if n.Line > 0 {
			if existing, ok := meta[n.Line]; !ok || existing.depth > depth {
				meta[n.Line] = lineMetadata{
					depth: depth, inSeq: true, seqOffset: seqOffset,
					sequenceIndentDepth: sequenceIndentDepth,
				}
			}
		}
		for _, item := range n.Content {
			// Record the start line of each sequence item at this depth.
			// yaml.v3 locates a nested sequence at its inner dash. Preserve the
			// parent depth only when an earlier dash shares that source line;
			// otherwise the recursive visit records the deeper depth.
			if item.Line > 0 && (item.Kind != yaml.SequenceNode ||
				hasEarlierSequenceDash(dashColumns[item.Line], item.Column)) {
				if existing, ok := meta[item.Line]; !ok || existing.depth > depth {
					meta[item.Line] = lineMetadata{
						depth: depth, inSeq: true, seqOffset: seqOffset,
						sequenceIndentDepth: sequenceIndentDepth,
					}
				}
			}
			if item.Kind == yaml.SequenceNode {
				// Nested sequence: inner items are one level deeper.
				// Don't increment seqOffset — the dash handles positioning.
				collectMetadata(item, meta, dashColumns, depth+1, true, seqOffset,
					sequenceIndentDepth)
			} else {
				collectMetadata(item, meta, dashColumns, depth, true, seqOffset,
					sequenceIndentDepth)
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// Bare sequence items (scalars/aliases not inside a mapping key/value
		// pair) need metadata so their lines get proper ASTDepth.
		if inSeq && n.Line > 0 {
			if existing, ok := meta[n.Line]; !ok || existing.depth > depth {
				meta[n.Line] = lineMetadata{
					depth: depth, inSeq: true, seqOffset: seqOffset,
					sequenceIndentDepth: sequenceIndentDepth,
				}
			}
		}
	default:
		// Unknown node kind — no metadata to collect.
	}
}

func sequenceDashColumns(tokens []Token) map[int][]int {
	columns := make(map[int][]int)
	line, column := 1, 1
	for _, token := range tokens {
		if token.Kind == TokDash {
			columns[line] = append(columns[line], column)
		}
		for _, b := range token.Raw {
			if b == '\n' {
				line, column = line+1, 1
			} else {
				column++
			}
		}
	}
	return columns
}

func hasEarlierSequenceDash(columns []int, column int) bool {
	return slices.ContainsFunc(columns, func(dashColumn int) bool {
		return dashColumn < column
	})
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
		tokens[i].SequenceIndentDepth = lm.sequenceIndentDepth
		// Propagate to preceding indent token (reindent operates on indent tokens).
		indentIdx := findPrecedingIndent(tokens, i)
		if indentIdx >= 0 {
			tokens[indentIdx].ASTDepth = lm.depth
			tokens[indentIdx].InSeq = lm.inSeq
			tokens[indentIdx].SeqOffset = lm.seqOffset
			tokens[indentIdx].SequenceIndentDepth = lm.sequenceIndentDepth
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
			tokens[indentIdx].SequenceIndentDepth = lm.sequenceIndentDepth
		}
	}

	// Pass 3: Propagate metadata to standalone comment lines.
	// A comment's indentation is determined by context:
	// - "Leading" comments (before content at the same or deeper level) inherit
	//   from the NEXT structural indent.
	// - "End" comments (trailing after deeper content, before shallower content)
	//   inherit from the PREVIOUS structural indent.
	//
	// We use the comment's SOURCE indent as the discriminator:
	// - If the comment's source indent >= previous structural line's source indent
	//   AND > next structural line's source indent, it's an end comment.
	// - Otherwise it's a leading comment.
	for i := range tokens {
		if tokens[i].Kind != TokIndent || !tokens[i].Structural || tokens[i].ASTDepth >= 0 {
			continue
		}
		// Check if this indent precedes a comment.
		if i+1 >= len(tokens) || tokens[i+1].Kind != TokComment {
			continue
		}

		commentSourceIndent := len(tokens[i].Raw)

		// Find the previous structural indent with assigned ASTDepth.
		// Skip other comment lines to avoid cascading (only look at key/dash lines).
		var prevFound bool
		var prevSourceIndent int
		var prevASTDepth int
		var prevInSeq bool
		var prevSeqOffset int
		var prevSeqIndentDepth int
		var prevIdx int
		for j := i - 1; j >= 0; j-- {
			if tokens[j].Kind == TokIndent && tokens[j].Structural && tokens[j].ASTDepth >= 0 {
				// Skip if this indent precedes a comment (another comment line).
				if j+1 < len(tokens) && tokens[j+1].Kind == TokComment {
					continue
				}
				prevFound = true
				prevIdx = j
				prevSourceIndent = len(tokens[j].Raw)
				prevASTDepth = tokens[j].ASTDepth
				prevInSeq = tokens[j].InSeq
				prevSeqOffset = tokens[j].SeqOffset
				prevSeqIndentDepth = tokens[j].SequenceIndentDepth
				break
			}
		}

		// Find the next structural indent with assigned ASTDepth.
		// Skip other comment lines.
		var nextFound bool
		var nextSourceIndent int
		var nextASTDepth int
		var nextInSeq bool
		var nextSeqOffset int
		var nextSeqIndentDepth int
		var nextIdx int
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].Kind == TokIndent && tokens[j].Structural && tokens[j].ASTDepth >= 0 {
				// Skip if this indent precedes a comment.
				if j+1 < len(tokens) && tokens[j+1].Kind == TokComment {
					continue
				}
				nextFound = true
				nextSourceIndent = len(tokens[j].Raw)
				nextASTDepth = tokens[j].ASTDepth
				nextInSeq = tokens[j].InSeq
				nextSeqOffset = tokens[j].SeqOffset
				nextSeqIndentDepth = tokens[j].SequenceIndentDepth
				nextIdx = j
				break
			}
		}

		// Determine if this is an end comment or leading comment.
		// Use AST depth as the primary signal (stable across passes).
		// Source indentation is only a tiebreaker when depths are equal.
		isEndComment := false
		if prevFound && nextFound {
			// End comment when:
			// - Structure is descoping (prev deeper than next), OR
			// - Same depth but comment indented deeper than next (trailing after value block).
			// If prevASTDepth < nextASTDepth: structure is deepening → leading comment.
			if prevASTDepth > nextASTDepth ||
				(prevASTDepth == nextASTDepth && commentSourceIndent > nextSourceIndent) {
				isEndComment = true
			}
		} else if prevFound && !nextFound {
			// Comment at end of file: if indented deeper than root, it's an end comment.
			if commentSourceIndent > 0 {
				isEndComment = true
			}
		}

		if isEndComment {
			// End comment: inherit from the previous context.
			// If the comment's source indent matches the prev line's source indent
			// and prev is a sequence item (has dash), the comment is at dash level.
			if commentSourceIndent <= prevSourceIndent && prevInSeq && lineHasDash(tokens, prevIdx) {
				// Dash-level end comment (e.g. `# sequence` after `- 123`).
				// Set AtSeqItem so it computes to dash level (inSeq + hasDash).
				tokens[i].ASTDepth = prevASTDepth
				tokens[i].InSeq = prevInSeq
				tokens[i].SeqOffset = prevSeqOffset
				tokens[i].SequenceIndentDepth = prevSeqIndentDepth
				tokens[i].AtSeqItem = true
			} else if commentSourceIndent > prevSourceIndent {
				// Comment is deeper than prev structural line (it belongs to
				// the value block, one level deeper than the key/dash).
				tokens[i].ASTDepth = prevASTDepth + 1
				tokens[i].InSeq = false
				tokens[i].SeqOffset = prevSeqOffset
				tokens[i].SequenceIndentDepth = prevSeqIndentDepth
			} else {
				// Same indent as prev (non-sequence): inherit directly.
				tokens[i].ASTDepth = prevASTDepth
				tokens[i].InSeq = prevInSeq
				tokens[i].SeqOffset = prevSeqOffset
				tokens[i].SequenceIndentDepth = prevSeqIndentDepth
				tokens[i].AtSeqItem = false
			}
		} else if nextFound {
			// Leading comment: inherit from next structural line.
			tokens[i].ASTDepth = nextASTDepth
			tokens[i].InSeq = nextInSeq
			tokens[i].SeqOffset = nextSeqOffset
			tokens[i].SequenceIndentDepth = nextSeqIndentDepth
			// When the comment precedes a sequence-item dash, it should align
			// with the dash (item indent), not the item's content (base + 2).
			tokens[i].AtSeqItem = lineHasDash(tokens, nextIdx)
		} else if prevFound {
			// Only previous exists (end of file) — root level.
			tokens[i].ASTDepth = 0
			tokens[i].InSeq = false
			tokens[i].SeqOffset = 0
			tokens[i].SequenceIndentDepth = 0
		}
	}
}

// computeNewIndent returns the target indentation for a structural line.
//
//nolint:revive // inSeq/hasDash are structural properties, not control flags
func computeNewIndent(astDepth int, inSeq, hasDash bool, seqOffset,
	sequenceIndentDepth, targetWidth int, indentSequences bool) int {
	base := astDepth*targetWidth + seqOffset*2
	if !indentSequences {
		base -= sequenceIndentDepth * targetWidth
		if base < 0 {
			base = 0
		}
	}
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
	lines := make(map[int]bool)
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var root yaml.Node
		if err := dec.Decode(&root); err != nil {
			break
		}
		collectStructuralLines(&root, lines)
	}
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
func reindentTokens(tokens []Token, targetWidth int, indentSequences bool) {
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
		if tokens[i].Kind != TokKey && tokens[i].Kind != TokValue && tokens[i].Kind != TokFlow {
			continue
		}

		if tokens[i].Kind == TokKey || tokens[i].Kind == TokValue {
			// Block scalar quote conversion.
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
			continue
		}

		// TokFlow: convert quoted scalars inside flow collections.
		tokens[i].Raw = convertFlowQuotes(tokens[i].Raw, style)
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

// convertFlowQuotes converts quoted scalars inside a flow collection token
// (TokFlow) to the target quote style. Walks the raw bytes, finds quoted
// scalar boundaries, and applies convertQuote to each one individually.
func convertFlowQuotes(raw []byte, style formatter.QuoteStyle) []byte {
	var result []byte
	i := 0
	for i < len(raw) {
		switch raw[i] {
		case '\'':
			// Find the end of this single-quoted scalar.
			end := findSingleQuoteEnd(raw, i)
			if end < 0 {
				// Malformed — copy rest verbatim.
				result = append(result, raw[i:]...)
				return result
			}
			scalar := raw[i : end+1]
			if isSimpleQuoted(scalar, '\'') {
				converted := convertQuote(scalar, style, '\'')
				result = append(result, converted...)
			} else {
				result = append(result, scalar...)
			}
			i = end + 1
		case '"':
			// Find the end of this double-quoted scalar.
			end := findDoubleQuoteEnd(raw, i)
			if end < 0 {
				result = append(result, raw[i:]...)
				return result
			}
			scalar := raw[i : end+1]
			if isSimpleQuoted(scalar, '"') {
				converted := convertQuote(scalar, style, '"')
				result = append(result, converted...)
			} else {
				result = append(result, scalar...)
			}
			i = end + 1
		default:
			result = append(result, raw[i])
			i++
		}
	}
	return result
}

// findSingleQuoteEnd finds the closing ' of a single-quoted YAML scalar.
// In single-quoted YAML, ” is an escape for a literal '.
// Returns the index of the closing quote, or -1 if not found.
func findSingleQuoteEnd(raw []byte, start int) int {
	// start points to the opening '
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\'' {
			// Check if it's an escape ('')
			if i+1 < len(raw) && raw[i+1] == '\'' {
				i++ // skip the escape pair
				continue
			}
			return i
		}
	}
	return -1
}

// findDoubleQuoteEnd finds the closing " of a double-quoted YAML scalar.
// Handles backslash escapes (\" is not the end).
// Returns the index of the closing quote, or -1 if not found.
func findDoubleQuoteEnd(raw []byte, start int) int {
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '\\' {
			i++ // skip escaped character
			continue
		}
		if raw[i] == '"' {
			return i
		}
	}
	return -1
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

func normalizeSequenceSpacing(tokens []Token) {
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Kind == TokSpace && tokens[i-1].Kind == TokDash {
			tokens[i].Raw = nil
		}
	}
}

// =============================================================================
// Flow collection normalization
// =============================================================================

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

// =============================================================================
// Utilities
// =============================================================================

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

// endsWithBlockScalarPreservingNewlines checks whether the last meaningful token
// is a block scalar whose trailing newlines are semantically significant.
// Only |- (strip) allows removal. Both | (clip) and |+ (keep) preserve newlines.
func endsWithBlockScalarPreservingNewlines(tokens []Token) bool {
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i].Kind {
		case TokNewline, TokIndent, TokSpace:
			continue
		case TokBlockScalar:
			return !blockScalarHasStripChomping(tokens[i].Raw)
		default:
			return false
		}
	}
	return false
}

// blockScalarHasStripChomping checks if a block scalar header contains '-' (strip indicator).
// Only scans the indicator portion of the header (before any comment).
func blockScalarHasStripChomping(raw []byte) bool {
	nlIdx := bytes.IndexByte(raw, '\n')
	if nlIdx < 0 {
		return false
	}
	// Scan header characters before comment (space/tab + # starts comment).
	for _, b := range raw[:nlIdx] {
		if b == ' ' || b == '\t' || b == '#' {
			break
		}
		if b == '-' {
			return true
		}
	}
	return false
}
