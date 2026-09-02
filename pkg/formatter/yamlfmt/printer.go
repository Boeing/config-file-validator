// printer.go — Phase pipeline orchestrator.
//
// printFormatted runs formatting phases in a fixed order. Each phase reads
// token fields set by earlier phases and writes fields consumed by later
// phases. The ordering constraints are:
//
//  1. buildASTMetadata + annotate — must run first. Sets Structural, Line,
//     ASTDepth, InSeq, SeqOffset, AtSeqItem, SequenceIndentDepth.
//  2. normalizeFlowTokens / normalizeValueSpacing — no annotation dependency,
//     but must run before reindent so spacing is settled.
//  3. sortKeys — reads ASTDepth; must run before reindent. Returns new
//     AnnotatedTokens with metadata intact.
//  4. reindentTokens — reads ASTDepth, InSeq, SeqOffset. Writes Token.Raw
//     on TokIndent and TokBlockScalar.
//  5. Post-reindent phases (trimBlockScalarTrailingBlanks, applyQuoteStyle,
//     comment spacing, normalizeSequenceSpacing, expandLongFlowSequences,
//     blank-line collapsing, serializeWithStrip) — operate on raw bytes only.
package yamlfmt

import (
	"bytes"

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
	// Returns AnnotatedTokens — phases that depend on annotation data
	// (reindentTokens, sortKeys) accept this type, not raw []Token.
	at := annotate(tokens, src, astMeta)

	// --- Phases that don't need annotation guarantees use at.Tokens() ---

	// Match Prettier's bracketSpacing behavior for flow mappings without
	// rewriting the user's colon or comma spacing.
	normalizeFlowTokens(at.Tokens())

	// Normalize value spacing: strip leading whitespace from values after colons.
	normalizeValueSpacing(at.Tokens())

	// --- Annotation-dependent phases (compiler-enforced via AnnotatedTokens) ---

	// Sort keys if requested. Metadata travels with tokens.
	if opts.SortKeys {
		at = sortKeys(at)
		// No depth recomputation needed — ASTDepth is position-invariant.
	}

	// Reindent: structural tokens get depth×width; continuations shift by parent delta.
	reindentTokens(at, targetWidth,
		opts.IndentSequences != formatter.SequenceIndentDisabled)

	// --- Post-reindent phases (annotation consumed, back to []Token) ---
	tokens = at.Tokens()

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

	// Strip blank lines before end-of-document trailing comments in sequence bodies.
	// prettier uses hardline (not hardline+hardline) between sequence content and
	// endComments. Mapping bodies preserve the blank line.
	tokens = stripBlankLinesBeforeEndComments(tokens)

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
