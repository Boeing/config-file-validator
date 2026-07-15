package xmlfmt

// TokenKind identifies the type of an XML token.
type TokenKind int

const (
	// TokIndent is leading whitespace at the start of a line.
	TokIndent TokenKind = iota
	// TokNewline is a line ending (\n or \r\n).
	TokNewline
	// TokXMLDecl is an XML declaration (<?xml ...?>).
	TokXMLDecl
	// TokDoctype is a DOCTYPE declaration (<!DOCTYPE ...>).
	TokDoctype
	// TokProcInst is a processing instruction (<?target ...?>).
	TokProcInst
	// TokComment is an XML comment (<!-- ... -->).
	TokComment
	// TokCDATA is a CDATA section (<![CDATA[...]]>).
	TokCDATA
	// TokOpenTag is an opening element tag (<element ...>).
	TokOpenTag
	// TokCloseTag is a closing element tag (</element>).
	TokCloseTag
	// TokSelfClose is a self-closing element tag (<element .../>).
	TokSelfClose
	// TokText is text content between tags.
	TokText
)

// Token represents a single lexical token in an XML file.
type Token struct {
	Kind       TokenKind
	Raw        []byte
	Structural bool // true if this indent precedes a tag (reindent); false for text content
	Depth      int  // structural depth for TokIndent tokens; -1 for others
}

// tokenize lexes XML source into a flat token stream.
// Every byte in src is accounted for in exactly one token.
func tokenize(src []byte) []Token {
	t := &xmlTokenizer{src: src}
	t.run()
	return t.tokens
}

type xmlTokenizer struct {
	src         []byte
	pos         int
	tokens      []Token
	atLineStart bool
}

func (t *xmlTokenizer) emit(kind TokenKind, start int) {
	if t.pos > start {
		t.tokens = append(t.tokens, Token{Kind: kind, Raw: t.src[start:t.pos], Depth: -1})
	}
}

func (t *xmlTokenizer) run() {
	t.atLineStart = true
	for t.pos < len(t.src) {
		if t.atLineStart {
			t.consumeLineStart()
		} else {
			t.consumeContent()
		}
	}
}

// consumeLineStart handles leading whitespace as TokIndent.
func (t *xmlTokenizer) consumeLineStart() {
	start := t.pos
	for t.pos < len(t.src) && (t.src[t.pos] == ' ' || t.src[t.pos] == '\t') {
		t.pos++
	}
	if t.pos > start {
		t.emit(TokIndent, start)
	}
	t.atLineStart = false
}

// consumeContent handles tokens after indent.
func (t *xmlTokenizer) consumeContent() {
	if t.pos >= len(t.src) {
		return
	}

	switch { //nolint:staticcheck // QF1002: tagged switch would lose the multi-char prefix checks
	case t.src[t.pos] == '\n':
		t.pos++
		t.emit(TokNewline, t.pos-1)
		t.atLineStart = true

	case t.src[t.pos] == '\r':
		start := t.pos
		t.pos++
		if t.pos < len(t.src) && t.src[t.pos] == '\n' {
			t.pos++
		}
		t.emit(TokNewline, start)
		t.atLineStart = true

	case t.src[t.pos] == '<':
		t.consumeTag()

	default:
		// Text content between tags.
		t.consumeText()
	}
}

// consumeTag classifies and consumes a tag starting with '<'.
func (t *xmlTokenizer) consumeTag() {
	start := t.pos

	if t.startsWith("<!--") {
		t.consumeComment(start)
	} else if t.startsWith("<![CDATA[") {
		t.consumeCDATA(start)
	} else if t.startsWith("<!DOCTYPE") || t.startsWith("<!doctype") {
		t.consumeDoctype(start)
	} else if t.startsWith("<?xml") || t.startsWith("<?XML") {
		// Only treat as XMLDecl if followed by whitespace or '?' — not if it's
		// part of a longer PI name like <?xml-stylesheet...?>.
		pos5 := t.pos + 5
		if pos5 >= len(t.src) || t.src[pos5] == ' ' || t.src[pos5] == '\t' ||
			t.src[pos5] == '\r' || t.src[pos5] == '\n' || t.src[pos5] == '?' {
			t.consumeXMLDecl(start)
		} else {
			t.consumeProcInst(start)
		}
	} else if t.startsWith("<?") {
		t.consumeProcInst(start)
	} else if t.startsWith("</") { //nolint:revive // max-control-nesting: sequential prefix checks require this depth
		t.consumeCloseTag(start)
	} else {
		t.consumeOpenOrSelfClose(start)
	}
}

// consumeComment consumes <!-- ... -->.
func (t *xmlTokenizer) consumeComment(start int) {
	t.pos += 4 // skip <!--
	for t.pos < len(t.src) {
		if t.pos+2 < len(t.src) && t.src[t.pos] == '-' && t.src[t.pos+1] == '-' && t.src[t.pos+2] == '>' {
			t.pos += 3
			t.emit(TokComment, start)
			return
		}
		t.pos++
	}
	// Unclosed comment — emit what we have.
	t.emit(TokComment, start)
}

// consumeCDATA consumes <![CDATA[...]]>.
func (t *xmlTokenizer) consumeCDATA(start int) {
	t.pos += 9 // skip <![CDATA[
	for t.pos < len(t.src) {
		if t.pos+2 < len(t.src) && t.src[t.pos] == ']' && t.src[t.pos+1] == ']' && t.src[t.pos+2] == '>' {
			t.pos += 3
			t.emit(TokCDATA, start)
			return
		}
		t.pos++
	}
	t.emit(TokCDATA, start)
}

// consumeDoctype consumes <!DOCTYPE ...> (handles nested brackets for internal subset).
func (t *xmlTokenizer) consumeDoctype(start int) {
	t.pos += 9 // skip <!DOCTYPE
	depth := 1 // track nested < > for internal DTD subset
	for t.pos < len(t.src) {
		switch t.src[t.pos] {
		case '[':
			// Internal subset — consume until ]
			t.pos++
			for t.pos < len(t.src) && t.src[t.pos] != ']' {
				t.pos++
			}
			if t.pos < len(t.src) {
				t.pos++ // skip ]
			}
		case '>':
			depth--
			t.pos++
			if depth == 0 {
				t.emit(TokDoctype, start)
				return
			}
		case '<':
			depth++
			t.pos++
		default:
			t.pos++
		}
	}
	t.emit(TokDoctype, start)
}

// consumeXMLDecl consumes <?xml ...?>.
func (t *xmlTokenizer) consumeXMLDecl(start int) {
	t.pos += 5 // skip <?xml
	for t.pos < len(t.src) {
		if t.pos+1 < len(t.src) && t.src[t.pos] == '?' && t.src[t.pos+1] == '>' {
			t.pos += 2
			t.emit(TokXMLDecl, start)
			return
		}
		t.pos++
	}
	t.emit(TokXMLDecl, start)
}

// consumeProcInst consumes <?target ...?>.
func (t *xmlTokenizer) consumeProcInst(start int) {
	t.pos += 2 // skip <?
	for t.pos < len(t.src) {
		if t.pos+1 < len(t.src) && t.src[t.pos] == '?' && t.src[t.pos+1] == '>' {
			t.pos += 2
			t.emit(TokProcInst, start)
			return
		}
		t.pos++
	}
	t.emit(TokProcInst, start)
}

// consumeCloseTag consumes </element>.
func (t *xmlTokenizer) consumeCloseTag(start int) {
	t.pos += 2 // skip </
	for t.pos < len(t.src) && t.src[t.pos] != '>' {
		t.pos++
	}
	if t.pos < len(t.src) {
		t.pos++ // skip >
	}
	t.emit(TokCloseTag, start)
}

// consumeOpenOrSelfClose consumes <element ...> or <element .../>.
// Must handle quoted attribute values containing >.
func (t *xmlTokenizer) consumeOpenOrSelfClose(start int) {
	t.pos++ // skip <
	for t.pos < len(t.src) {
		switch t.src[t.pos] {
		case '"':
			t.pos++
			for t.pos < len(t.src) && t.src[t.pos] != '"' {
				t.pos++
			}
			if t.pos < len(t.src) {
				t.pos++ // skip closing "
			}
		case '\'':
			t.pos++
			for t.pos < len(t.src) && t.src[t.pos] != '\'' {
				t.pos++
			}
			if t.pos < len(t.src) {
				t.pos++ // skip closing '
			}
		case '/':
			if t.pos+1 < len(t.src) && t.src[t.pos+1] == '>' {
				t.pos += 2
				t.emit(TokSelfClose, start)
				return
			}
			t.pos++
		case '>':
			t.pos++
			t.emit(TokOpenTag, start)
			return
		default:
			t.pos++
		}
	}
	// Unclosed tag — emit as open.
	t.emit(TokOpenTag, start)
}

// consumeText consumes text content until next < or newline.
func (t *xmlTokenizer) consumeText() {
	start := t.pos
	for t.pos < len(t.src) && t.src[t.pos] != '<' && t.src[t.pos] != '\n' && t.src[t.pos] != '\r' {
		t.pos++
	}
	if t.pos > start {
		t.emit(TokText, start)
	}
}

// startsWith checks if the remaining bytes start with the given prefix.
func (t *xmlTokenizer) startsWith(prefix string) bool {
	if t.pos+len(prefix) > len(t.src) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		// Case-insensitive for first char of known prefixes isn't needed —
		// we handle both cases explicitly in consumeTag.
		if t.src[t.pos+i] != prefix[i] {
			return false
		}
	}
	return true
}
