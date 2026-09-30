// Annotation phase: sets Structural, Line, ASTDepth, InSeq, SeqOffset, AtSeqItem, SequenceIndentDepth on tokens.
package yamlfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"gopkg.in/yaml.v3"
)

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
func annotate(tokens []Token, src []byte, astMeta map[int]lineMetadata) AnnotatedTokens {
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

	return AnnotatedTokens{tokens: tokens}
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
