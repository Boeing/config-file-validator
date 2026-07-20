package yamlfmt

// TokenKind identifies the type of a YAML token.
type TokenKind int

const (
	// TokIndent is leading spaces at the start of a line.
	// This is the ONLY token modified during formatting.
	TokIndent TokenKind = iota
	// TokNewline is a line ending (\n or \r\n).
	TokNewline
	// TokComment is a comment (# through end of line, not including newline).
	TokComment
	// TokDash is a sequence entry indicator (- followed by space or newline).
	TokDash
	// TokKey is the key portion of a mapping pair (bare, single-quoted, or double-quoted).
	TokKey
	// TokColon is the : separator (with optional trailing space).
	TokColon
	// TokValue is a scalar value (inline after colon, or continuation lines).
	TokValue
	// TokDocStart is a document start marker (---).
	TokDocStart
	// TokDocEnd is a document end marker (...).
	TokDocEnd
	// TokTag is a tag (!, !!, or named tag).
	TokTag
	// TokAnchor is an anchor definition (&name).
	TokAnchor
	// TokAlias is an alias reference (*name).
	TokAlias
	// TokBlockScalar is a block scalar header + all content lines (opaque).
	// Content indentation is relative and must shift with the scalar's indent.
	TokBlockScalar
	// TokFlow is a flow collection ({...} or [...]) including delimiters (opaque).
	TokFlow
	// TokDirective is a directive line (%YAML or %TAG, not including newline).
	TokDirective
	// TokSpace is horizontal whitespace between tokens on the same line
	// (NOT leading indent — that's TokIndent).
	TokSpace
)

// Token represents a single lexical token in a YAML file.
// The Raw field preserves the original bytes exactly.
// Structural, Line, ASTDepth, and InSeq are set by annotate() after tokenization.
type Token struct {
	Kind       TokenKind
	Raw        []byte
	Structural bool // true if this indent should be renormalized
	Line       int  // 1-based source line number
	ASTDepth   int  // mapping nesting depth from AST. -1 = not annotated.
	InSeq      bool // true if this key is inside a sequence item (dash-relative indent)
	SeqOffset  int  // number of ancestor non-dash sequence levels contributing +2 each
	AtSeqItem  bool // true for a standalone comment that precedes a sequence-item dash
}

// tokenize lexes YAML source into a flat token stream.
// Every byte in src is accounted for in exactly one token.
// The tokenizer is format-only — it classifies tokens and preserves boundaries.
// It does NOT decode values, resolve anchors, or validate semantics.
func tokenize(src []byte) []Token {
	t := &tokenizer{src: src}
	t.run()
	return t.tokens
}

// tokenizer holds the state for lexing YAML source.
type tokenizer struct {
	src    []byte
	pos    int
	tokens []Token
	// atLineStart tracks whether we're at the beginning of a line
	// (immediately after a newline or at the start of input).
	atLineStart bool
	// lineHasStructure is true if the current line has emitted a TokColon
	// or TokDash. Used to determine if |/> can be a block scalar header
	// (only valid in value position on a line with structural markers).
	lineHasStructure bool
}

// emit appends a token to the output.
func (t *tokenizer) emit(kind TokenKind, start int) {
	if t.pos > start {
		t.tokens = append(t.tokens, Token{Kind: kind, Raw: t.src[start:t.pos]})
	}
}

// remaining returns the number of bytes left.
func (t *tokenizer) remaining() int {
	return len(t.src) - t.pos
}

// run is the main tokenizer loop.
func (t *tokenizer) run() {
	t.atLineStart = true

	for t.pos < len(t.src) {
		if t.atLineStart {
			t.consumeLineStart()
		} else {
			t.consumeLineContent()
		}
	}
}

// consumeLineStart handles the beginning of a line: indent + dispatch.
func (t *tokenizer) consumeLineStart() {
	// Consume leading spaces as indent.
	start := t.pos
	for t.pos < len(t.src) && t.src[t.pos] == ' ' {
		t.pos++
	}
	if t.pos > start {
		t.emit(TokIndent, start)
	}

	// End of input after indent.
	if t.pos >= len(t.src) {
		return
	}

	t.atLineStart = false

	// Dispatch based on first non-space character.
	switch {
	case t.src[t.pos] == '\n' || t.src[t.pos] == '\r':
		// Blank line — the newline will be consumed by consumeLineContent.
		// (atLineStart is already false, consumeLineContent will handle it.)

	case t.src[t.pos] == '#':
		t.consumeComment()

	case t.src[t.pos] == '%' && t.indentLevel(start) == 0:
		t.consumeDirective()

	case t.matchDocMarker(start):
		// Handled inside matchDocMarker.

	default:
		// Regular line content — handled by consumeLineContent on next iteration.
	}
}

// consumeLineContent handles tokens within a line (after indent).
func (t *tokenizer) consumeLineContent() {
	if t.pos >= len(t.src) {
		return
	}

	switch b := t.src[t.pos]; b {
	case '\n':
		t.pos++
		t.emit(TokNewline, t.pos-1)
		t.atLineStart = true
		t.lineHasStructure = false

	case '\r':
		start := t.pos
		t.pos++
		if t.pos < len(t.src) && t.src[t.pos] == '\n' {
			t.pos++
		}
		t.emit(TokNewline, start)
		t.atLineStart = true
		t.lineHasStructure = false

	case '#':
		t.consumeComment()

	case ' ', '\t':
		t.consumeSpace()

	default:
		// Check for sequence entry (- followed by space or newline).
		if b == '-' && t.pos+1 < len(t.src) && (t.src[t.pos+1] == ' ' || t.src[t.pos+1] == '\n' || t.src[t.pos+1] == '\r') {
			start := t.pos
			t.pos++ // consume the dash
			if t.pos < len(t.src) && t.src[t.pos] == ' ' {
				t.pos++ // include trailing space in TokDash ("- ")
			}
			// If followed by newline, DON'T consume it — let the newline
			// handler set atLineStart so the next line's indent is TokIndent.
			t.emit(TokDash, start)
			t.lineHasStructure = true
			return
		}
		if b == '-' && t.pos+1 >= len(t.src) {
			// Dash at EOF — sequence entry.
			start := t.pos
			t.pos++
			t.emit(TokDash, start)
			t.lineHasStructure = true
			return
		}
		// Anchor, alias, tag at start of content.
		if b == '&' {
			t.consumeAnchor()
			return
		}
		if b == '*' {
			t.consumeAlias()
			return
		}
		if b == '!' {
			t.consumeTag()
			return
		}
		// Check for flow collections.
		if b == '{' || b == '[' {
			t.consumeFlowCollection()
			return
		}
		// Check for block scalar indicators (| or >).
		// Only valid as block scalar headers on lines that have structure
		// (a colon or dash earlier on this line). On continuation lines
		// without structure, |/> is a plain scalar value.
		if (b == '|' || b == '>') && t.lineHasStructure && t.isBlockScalarStart() {
			t.consumeBlockScalar()
		} else {
			// Key/value content.
			t.consumeRestOfLine()
		}
	}
}

// consumeComment consumes from # through end of line (not including newline).
func (t *tokenizer) consumeComment() {
	start := t.pos
	for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
		t.pos++
	}
	t.emit(TokComment, start)
}

// consumeDirective consumes from % through end of line (not including newline).
func (t *tokenizer) consumeDirective() {
	start := t.pos
	for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
		t.pos++
	}
	t.emit(TokDirective, start)
}

// consumeSpace consumes horizontal whitespace (spaces and tabs) between tokens.
func (t *tokenizer) consumeSpace() {
	start := t.pos
	for t.pos < len(t.src) && (t.src[t.pos] == ' ' || t.src[t.pos] == '\t') {
		t.pos++
	}
	t.emit(TokSpace, start)
}

// consumeRestOfLine consumes line content, splitting into key/colon/value
// tokens when a mapping separator (: followed by space/newline/EOF) is found.
// If no separator is found, the entire line is emitted as TokValue.
func (t *tokenizer) consumeRestOfLine() {
	start := t.pos

	// Find the colon separator (if any). A colon is a separator only if:
	// 1. It's outside of quotes
	// 2. It's followed by space, newline, or EOF
	// Quoted keys: scan the quoted string first, then look for colon after.
	colonPos := t.findColonSeparator(t.pos)

	if colonPos < 0 {
		// No colon separator — whole line is a value.
		for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
			t.pos++
		}
		t.emit(TokValue, start)
		return
	}

	// Emit key (everything before the colon).
	if colonPos > start {
		t.pos = colonPos
		t.emit(TokKey, start)
	}

	// Emit colon (: and optional trailing space).
	colonStart := t.pos
	t.pos++ // skip :
	if t.pos < len(t.src) && t.src[t.pos] == ' ' {
		t.pos++ // include the space after colon
	}
	t.emit(TokColon, colonStart)
	t.lineHasStructure = true

	// Remaining content on this line is the value (if any).
	// But first check for block scalar, flow collection, or other special starts.
	if t.pos >= len(t.src) || t.src[t.pos] == '\n' || t.src[t.pos] == '\r' {
		return // no value on this line
	}

	// Check for special value types.
	b := t.src[t.pos]
	switch {
	case (b == '|' || b == '>') && t.isBlockScalarStart():
		t.consumeBlockScalar()
	case b == '{' || b == '[':
		t.consumeFlowCollection()
	case b == '#':
		t.consumeComment()
	case b == '&':
		t.consumeAnchor()
	case b == '*':
		t.consumeAlias()
	case b == '!':
		t.consumeTag()
	default:
		// Value after colon. If the value starts with a quote AND the matching
		// close quote is followed by end-of-line/comment/space-comment, consume
		// as a quoted value. Otherwise treat as plain scalar.
		valStart := t.pos
		if t.src[t.pos] == '"' && t.isFullyQuotedDouble() {
			t.skipDoubleQuoted()
		} else if t.src[t.pos] == '\'' && t.isFullyQuotedSingle() {
			t.skipSingleQuoted()
		} else {
			// Unquoted value — consume to end of line, stop at inline comments.
			for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
				if t.src[t.pos] == ' ' && t.pos+1 < len(t.src) && t.src[t.pos+1] == '#' {
					break
				}
				t.pos++
			}
			// Trim trailing spaces from value (they become TokSpace before comment).
			for t.pos > valStart && t.src[t.pos-1] == ' ' {
				t.pos--
			}
		}
		if t.pos > valStart {
			t.emit(TokValue, valStart)
		}
	}
}

// findColonSeparator scans from position p to find an unquoted colon that's
// followed by space, newline, or EOF (making it a mapping separator).
// Returns the position of the colon, or -1 if not found.
func (t *tokenizer) findColonSeparator(p int) int {
	for p < len(t.src) && t.src[p] != '\n' && t.src[p] != '\r' {
		b := t.src[p]
		switch b {
		case '"':
			// Skip double-quoted string.
			p++
			for p < len(t.src) && t.src[p] != '\n' && t.src[p] != '\r' {
				if t.src[p] == '\\' && p+1 < len(t.src) {
					p += 2
				} else if t.src[p] == '"' {
					p++
					break
				} else {
					p++
				}
			}
		case '\'':
			// Skip single-quoted string.
			p++
			for p < len(t.src) && t.src[p] != '\n' && t.src[p] != '\r' {
				if t.src[p] == '\'' {
					p++
					if p >= len(t.src) || t.src[p] != '\'' {
						break
					}
					p++ // escaped ''
				} else {
					p++
				}
			}
		case ':':
			// Check if followed by space, newline, or EOF.
			if p+1 >= len(t.src) || t.src[p+1] == ' ' || t.src[p+1] == '\n' || t.src[p+1] == '\r' || t.src[p+1] == '\t' {
				return p
			}
			p++
		default:
			p++
		}
	}
	return -1
}

// consumeAnchor consumes &name as TokAnchor.
func (t *tokenizer) consumeAnchor() {
	start := t.pos
	t.pos++ // skip &
	for t.pos < len(t.src) && !isYAMLWhitespace(t.src[t.pos]) && t.src[t.pos] != ',' && t.src[t.pos] != ']' && t.src[t.pos] != '}' {
		t.pos++
	}
	t.emit(TokAnchor, start)
}

// consumeAlias consumes *name as TokAlias.
func (t *tokenizer) consumeAlias() {
	start := t.pos
	t.pos++ // skip *
	for t.pos < len(t.src) && !isYAMLWhitespace(t.src[t.pos]) && t.src[t.pos] != ',' && t.src[t.pos] != ']' && t.src[t.pos] != '}' {
		t.pos++
	}
	t.emit(TokAlias, start)
}

// consumeTag consumes a tag (!tag or !!tag or !<tag>) as TokTag.
func (t *tokenizer) consumeTag() {
	start := t.pos
	t.pos++ // skip !
	if t.pos < len(t.src) && t.src[t.pos] == '<' {
		// Verbatim tag: !<...>
		for t.pos < len(t.src) && t.src[t.pos] != '>' {
			t.pos++
		}
		if t.pos < len(t.src) {
			t.pos++ // skip >
		}
	} else {
		// Named tag: !! or !name!suffix or !suffix
		for t.pos < len(t.src) && !isYAMLWhitespace(t.src[t.pos]) {
			t.pos++
		}
	}
	t.emit(TokTag, start)
}

// isYAMLWhitespace returns true for space, tab, newline, or carriage return.
func isYAMLWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isFullyQuotedDouble checks if the double-quoted string at t.pos spans to
// a closing " that is followed by EOL, EOF, or space+#. If not, the quote
// is part of a plain scalar (e.g., "foo"bar).
func (t *tokenizer) isFullyQuotedDouble() bool {
	p := t.pos + 1 // skip opening "
	for p < len(t.src) {
		if t.src[p] == '\\' && p+1 < len(t.src) {
			p += 2 // skip escape
		} else if t.src[p] == '"' {
			p++ // skip closing "
			// Check what follows.
			if p >= len(t.src) || t.src[p] == '\n' || t.src[p] == '\r' {
				return true
			}
			if t.src[p] == ' ' {
				return true // space before comment or end of value
			}
			return false // characters after close quote → plain scalar
		} else if t.src[p] == '\n' || t.src[p] == '\r' {
			return true // multi-line quoted string — treat as quoted
		} else {
			p++
		}
	}
	return false // unclosed quote
}

// isFullyQuotedSingle checks if the single-quoted string at t.pos spans to
// a closing ' that is followed by EOL, EOF, or space+#.
func (t *tokenizer) isFullyQuotedSingle() bool {
	p := t.pos + 1 // skip opening '
	for p < len(t.src) {
		switch t.src[p] {
		case '\'':
			p++
			if p >= len(t.src) || t.src[p] != '\'' {
				// Closing quote. Check what follows.
				if p >= len(t.src) || t.src[p] == '\n' || t.src[p] == '\r' {
					return true
				}
				if t.src[p] == ' ' {
					return true
				}
				return false // characters after close quote → plain scalar
			}
			p++ // escaped '' — continue
		case '\n', '\r':
			return true // multi-line quoted string
		default:
			p++
		}
	}
	return false // unclosed quote
}

// consumeBlockScalar consumes a block scalar: the header line (| or >)
// plus all content lines. The entire block is one opaque TokBlockScalar.
//
// Block scalar rules:
//   - Header: [|>] [indent-indicator] [chomping-indicator] [comment]
//   - The indent indicator (1-9) explicitly sets content indent.
//   - Without an indicator, the first non-empty content line determines indent.
//   - Content continues until a non-empty line has fewer leading spaces than
//     the content indent level, or until a document marker (---/...) at column 0.
//   - Empty lines (and lines with only spaces) within the block are part of it.
func (t *tokenizer) consumeBlockScalar() {
	start := t.pos

	// Consume the header line (|/> through end of line, not including newline).
	t.pos++ // skip | or >

	// Parse optional indent indicator from header.
	explicitIndent := 0
	for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
		b := t.src[t.pos]
		if b >= '1' && b <= '9' && explicitIndent == 0 {
			explicitIndent = int(b - '0')
		}
		t.pos++
	}

	// Consume the newline after the header (it's part of the block scalar token).
	if t.pos < len(t.src) {
		if t.src[t.pos] == '\r' && t.pos+1 < len(t.src) && t.src[t.pos+1] == '\n' {
			t.pos += 2
		} else if t.src[t.pos] == '\n' || t.src[t.pos] == '\r' {
			t.pos++
		}
	}

	// Determine content indent.
	// If explicit indicator given: content indent = parent indent + indicator.
	// Otherwise: first non-empty content line determines it.
	parentIndent := t.lastIndentLevel()
	contentIndent := 0

	if explicitIndent > 0 {
		contentIndent = parentIndent + explicitIndent
	} else {
		// Scan ahead (without advancing pos) to find first non-empty line's indent.
		contentIndent = t.detectBlockContentIndent(t.pos)
		if contentIndent <= parentIndent {
			// No content or content at/below parent — scalar is empty.
			t.emit(TokBlockScalar, start)
			t.atLineStart = true
			return
		}
	}

	// Consume content lines: any line that is empty OR has indent >= contentIndent.
	for t.pos < len(t.src) {
		// Check for document markers at column 0 — these terminate block scalars.
		if t.isDocMarkerAt(t.pos) {
			break
		}

		// Count leading spaces on this line.
		lineStart := t.pos
		spaces := 0
		for t.pos < len(t.src) && t.src[t.pos] == ' ' {
			spaces++
			t.pos++
		}

		// Check if line is empty (only whitespace + newline or at EOF).
		if t.pos >= len(t.src) {
			// Trailing spaces at EOF — part of block.
			break
		}
		if t.src[t.pos] == '\n' || t.src[t.pos] == '\r' {
			// Empty line — consume newline, continue (it's part of the block).
			if t.src[t.pos] == '\r' && t.pos+1 < len(t.src) && t.src[t.pos+1] == '\n' {
				t.pos += 2
			} else {
				t.pos++
			}
			continue
		}

		// Non-empty line: check if indent is sufficient.
		if spaces < contentIndent {
			// This line is less indented — block scalar ends.
			// Rewind: the spaces (and content) belong to the next token.
			t.pos = lineStart
			break
		}

		// Line is part of the block — consume through end of line + newline.
		for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
			t.pos++
		}
		if t.pos < len(t.src) {
			if t.src[t.pos] == '\r' && t.pos+1 < len(t.src) && t.src[t.pos+1] == '\n' {
				t.pos += 2
			} else if t.src[t.pos] == '\n' || t.src[t.pos] == '\r' {
				t.pos++
			}
		}
	}

	t.emit(TokBlockScalar, start)
	t.atLineStart = true
}

// detectBlockContentIndent scans from position p to find the first non-empty
// line's indent level. Returns 0 if no non-empty line is found.
func (t *tokenizer) detectBlockContentIndent(p int) int {
	for p < len(t.src) {
		spaces := 0
		for p < len(t.src) && t.src[p] == ' ' {
			spaces++
			p++
		}
		if p < len(t.src) && t.src[p] != '\n' && t.src[p] != '\r' {
			return spaces
		}
		// Empty line — skip to next.
		if p < len(t.src) {
			if t.src[p] == '\r' && p+1 < len(t.src) && t.src[p+1] == '\n' {
				p += 2
			} else if t.src[p] == '\n' || t.src[p] == '\r' {
				p++
			}
		}
	}
	return 0
}

// lastIndentLevel returns the indent level of the most recent TokIndent, or 0.
func (t *tokenizer) lastIndentLevel() int {
	for i := len(t.tokens) - 1; i >= 0; i-- {
		if t.tokens[i].Kind == TokIndent {
			return len(t.tokens[i].Raw)
		}
		if t.tokens[i].Kind == TokNewline {
			return 0
		}
	}
	return 0
}

// isDocMarkerAt checks if position p starts a document marker (--- or ...).
func (t *tokenizer) isDocMarkerAt(p int) bool {
	if p+2 >= len(t.src) {
		return false
	}
	if (t.src[p] == '-' && t.src[p+1] == '-' && t.src[p+2] == '-') ||
		(t.src[p] == '.' && t.src[p+1] == '.' && t.src[p+2] == '.') {
		if p+3 >= len(t.src) || t.src[p+3] == ' ' || t.src[p+3] == '\n' || t.src[p+3] == '\r' {
			return true
		}
	}
	return false
}

// isBlockScalarStart checks whether the | or > at the current position is
// a block scalar header. A block scalar header consists of | or > followed
// only by optional [1-9], [+-], space, and #comment until end of line.
// If any other non-whitespace character follows on the same line, it's a
// plain scalar containing | or >.
func (t *tokenizer) isBlockScalarStart() bool {
	p := t.pos + 1 // skip the | or >
	for p < len(t.src) {
		b := t.src[p]
		switch {
		case b == '\n' || b == '\r':
			return true
		case b == ' ' || b == '\t':
			p++
		case b == '#':
			return true // comment follows — valid header
		case b == '+' || b == '-':
			p++
		// YAML spec allows digits 1-9 as explicit indentation indicator.
		case b >= '1' && b <= '9':
			p++
		default:
			return false // non-header character — not a block scalar
		}
	}
	// Reached EOF — valid (empty block scalar).
	return true
}

// consumeFlowCollection consumes a flow collection ({...} or [...]) as a
// single opaque TokFlow token. Handles nested braces/brackets and quoted
// strings (to avoid counting brackets inside strings).
func (t *tokenizer) consumeFlowCollection() {
	start := t.pos
	open := t.src[t.pos]
	var closeCh byte
	if open == '{' {
		closeCh = '}'
	} else {
		closeCh = ']'
	}
	depth := 1
	t.pos++ // skip opening delimiter

	for t.pos < len(t.src) && depth > 0 {
		b := t.src[t.pos]
		switch b {
		case open:
			depth++
			t.pos++
		case closeCh:
			depth--
			t.pos++
		case '"':
			t.skipDoubleQuoted()
		case '\'':
			t.skipSingleQuoted()
		case '#':
			// Comment inside flow — consume to EOL (still part of flow token).
			for t.pos < len(t.src) && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
				t.pos++
			}
		default:
			t.pos++
		}
	}

	t.emit(TokFlow, start)
}

// skipDoubleQuoted advances past a double-quoted string (handling escapes).
// Assumes current position is at the opening ".
func (t *tokenizer) skipDoubleQuoted() {
	t.pos++ // skip opening "
	for t.pos < len(t.src) {
		if t.src[t.pos] == '\\' && t.pos+1 < len(t.src) {
			t.pos += 2 // skip escape sequence
		} else if t.src[t.pos] == '"' {
			t.pos++ // skip closing "
			return
		} else {
			t.pos++
		}
	}
}

// skipSingleQuoted advances past a single-quoted string.
// In YAML, single-quoted strings escape ' as ”.
func (t *tokenizer) skipSingleQuoted() {
	t.pos++ // skip opening '
	for t.pos < len(t.src) {
		if t.src[t.pos] == '\'' {
			t.pos++
			// '' is an escaped single quote — not the end.
			if t.pos >= len(t.src) || t.src[t.pos] != '\'' {
				return
			}
			t.pos++
		} else {
			t.pos++
		}
	}
}

// matchDocMarker checks for --- or ... at column 0 and emits the appropriate token.
// Returns true if a marker was matched.
func (t *tokenizer) matchDocMarker(indentStart int) bool {
	col := t.pos - indentStart
	if col != 0 {
		return false
	}
	if t.remaining() >= 3 {
		if t.src[t.pos] == '-' && t.src[t.pos+1] == '-' && t.src[t.pos+2] == '-' {
			// Must be followed by space, newline, or EOF to be a doc marker.
			if t.remaining() == 3 || t.src[t.pos+3] == ' ' || t.src[t.pos+3] == '\n' || t.src[t.pos+3] == '\r' {
				start := t.pos
				t.pos += 3
				t.emit(TokDocStart, start)
				return true
			}
		}
		if t.src[t.pos] == '.' && t.src[t.pos+1] == '.' && t.src[t.pos+2] == '.' {
			if t.remaining() == 3 || t.src[t.pos+3] == ' ' || t.src[t.pos+3] == '\n' || t.src[t.pos+3] == '\r' {
				start := t.pos
				t.pos += 3
				t.emit(TokDocEnd, start)
				return true
			}
		}
	}
	return false
}

// indentLevel returns the number of spaces between indentStart and current pos.
func (t *tokenizer) indentLevel(indentStart int) int {
	return t.pos - indentStart
}
