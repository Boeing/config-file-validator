// Package tomlfmt provides a Formatter for TOML files.
//
// This file implements the TOML tokenizer (lexer). It produces a flat token
// stream from source bytes where every byte is accounted for in exactly one
// token. The tokenizer is format-only — it classifies tokens and preserves
// their exact source representation but does not interpret values.
//
// String boundary detection (including multiline strings with up to 2 extra
// quotes) is ported from taplo's battle-tested lexer logic.
package tomlfmt

// TokenKind identifies the type of a lexical token.
type TokenKind int

const (
	// Whitespace is horizontal whitespace (spaces and tabs, not newlines).
	Whitespace TokenKind = iota
	// Newline is a line ending (\n or \r\n).
	Newline
	// Comment is a # comment through end of line (not including the newline).
	Comment
	// BareKey is an unquoted key segment [A-Za-z0-9_-]+.
	BareKey
	// BasicString is a double-quoted string "..." including the quotes.
	BasicString
	// MultiLineBasicString is a triple-double-quoted string """...""".
	MultiLineBasicString
	// LiteralString is a single-quoted string '...' including the quotes.
	LiteralString
	// MultiLineLiteralString is a triple-single-quoted string '''...'''.
	MultiLineLiteralString
	// Integer is a decimal, hex, oct, or bin integer literal.
	Integer
	// Float is a float literal including nan and inf.
	Float
	// Bool is true or false.
	Bool
	// DateTime is any date/time literal (offset, local, date-only, time-only).
	DateTime
	// Dot is a period character.
	Dot
	// Comma is a comma character.
	Comma
	// Equals is an equals sign.
	Equals
	// BracketOpen is [.
	BracketOpen
	// BracketClose is ].
	BracketClose
	// BraceOpen is {.
	BraceOpen
	// BraceClose is }.
	BraceClose
)

// Token represents a single lexical token in a TOML document.
// Raw contains the exact source bytes for this token.
type Token struct {
	Kind   TokenKind
	Raw    []byte
	Offset int
}

// Lexer tokenizes TOML source into a flat token stream.
type Lexer struct {
	src []byte
	pos int
}

// NewLexer creates a new Lexer for the given source bytes.
func NewLexer(src []byte) *Lexer {
	return &Lexer{src: src}
}

// Tokenize lexes the entire source and returns all tokens.
// Every byte in src is accounted for in exactly one token.
func (l *Lexer) Tokenize() []Token {
	// Heuristic: ~4 bytes per token for typical config files.
	tokens := make([]Token, 0, len(l.src)/4)
	for l.pos < len(l.src) {
		tok := l.next()
		tokens = append(tokens, tok)
	}
	return tokens
}

// next consumes and returns the next token from the source.
func (l *Lexer) next() Token {
	start := l.pos
	b := l.src[l.pos]

	switch {
	case b == '\n':
		l.pos++
		return Token{Kind: Newline, Raw: l.src[start:l.pos], Offset: start}

	case b == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n':
		l.pos += 2
		return Token{Kind: Newline, Raw: l.src[start:l.pos], Offset: start}

	case b == '\r':
		// Bare \r without \n — treat as whitespace.
		l.pos++
		return Token{Kind: Whitespace, Raw: l.src[start:l.pos], Offset: start}

	case b == ' ' || b == '\t':
		l.lexWhitespace()
		return Token{Kind: Whitespace, Raw: l.src[start:l.pos], Offset: start}

	case b == '#':
		l.lexComment()
		return Token{Kind: Comment, Raw: l.src[start:l.pos], Offset: start}

	case b == '.':
		l.pos++
		return Token{Kind: Dot, Raw: l.src[start:l.pos], Offset: start}

	case b == ',':
		l.pos++
		return Token{Kind: Comma, Raw: l.src[start:l.pos], Offset: start}

	case b == '=':
		l.pos++
		return Token{Kind: Equals, Raw: l.src[start:l.pos], Offset: start}

	case b == '[':
		l.pos++
		return Token{Kind: BracketOpen, Raw: l.src[start:l.pos], Offset: start}

	case b == ']':
		l.pos++
		return Token{Kind: BracketClose, Raw: l.src[start:l.pos], Offset: start}

	case b == '{':
		l.pos++
		return Token{Kind: BraceOpen, Raw: l.src[start:l.pos], Offset: start}

	case b == '}':
		l.pos++
		return Token{Kind: BraceClose, Raw: l.src[start:l.pos], Offset: start}

	case b == '"':
		return l.lexDoubleQuotedString(start)

	case b == '\'':
		return l.lexSingleQuotedString(start)

	default:
		return l.lexBareWord(start)
	}
}

// lexWhitespace consumes consecutive spaces and tabs.
func (l *Lexer) lexWhitespace() {
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		if b != ' ' && b != '\t' {
			break
		}
		l.pos++
	}
}

// lexComment consumes a # comment through end of line (not including newline).
func (l *Lexer) lexComment() {
	for l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			break
		}
		if l.src[l.pos] == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
			break
		}
		l.pos++
	}
}

// lexDoubleQuotedString handles " and """ strings.
func (l *Lexer) lexDoubleQuotedString(start int) Token {
	// Check for multiline (""")
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
		l.pos += 3 // skip opening """
		l.lexMultiLineBasicString()
		return Token{Kind: MultiLineBasicString, Raw: l.src[start:l.pos], Offset: start}
	}

	// Basic string
	l.pos++ // skip opening "
	l.lexBasicString()
	return Token{Kind: BasicString, Raw: l.src[start:l.pos], Offset: start}
}

// lexBasicString consumes content until unescaped closing ".
// Called after the opening " has been consumed.
func (l *Lexer) lexBasicString() {
	escaped := false
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		l.pos++

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' {
			escaped = true
			continue
		}

		if b == '"' {
			return
		}
	}
	// Unterminated string — lexer continues (validation catches this).
}

// lexMultiLineBasicString consumes content until closing """.
// Called after the opening """ has been consumed.
// Per TOML spec, up to 2 additional " before the closing """ are content.
// So """"" = content "" + close """, and """""" (6) is invalid.
func (l *Lexer) lexMultiLineBasicString() {
	escaped := false
	quoteCount := 0
	quotesFound := false

	for l.pos < len(l.src) {
		b := l.src[l.pos]

		if quotesFound {
			if b != '"' {
				// End of quote run. If we had ≥ 3, we're done.
				return
			}
			// More quotes — accumulate (up to 5 total = 2 content + 3 close).
			quoteCount++
			l.pos++
			if quoteCount >= 5 {
				// Max: 5 quotes after opening = """""" total which is the limit.
				return
			}
			continue
		}

		l.pos++

		if escaped {
			escaped = false
			quoteCount = 0
			continue
		}

		if b == '\\' {
			escaped = true
			quoteCount = 0
			continue
		}

		if b == '"' {
			quoteCount++
			if quoteCount >= 3 {
				quotesFound = true
			}
		} else {
			quoteCount = 0
		}
	}
	// Unterminated — lexer continues.
}

// lexSingleQuotedString handles ' and ”' strings.
func (l *Lexer) lexSingleQuotedString(start int) Token {
	// Check for multiline (''')
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == '\'' && l.src[l.pos+2] == '\'' {
		l.pos += 3 // skip opening '''
		l.lexMultiLineLiteralString()
		return Token{Kind: MultiLineLiteralString, Raw: l.src[start:l.pos], Offset: start}
	}

	// Literal string
	l.pos++ // skip opening '
	l.lexLiteralString()
	return Token{Kind: LiteralString, Raw: l.src[start:l.pos], Offset: start}
}

// lexLiteralString consumes content until closing '.
// No escape sequences in literal strings.
func (l *Lexer) lexLiteralString() {
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		l.pos++
		if b == '\'' {
			return
		}
		// Literal strings cannot span lines (only multiline can).
		if b == '\n' || (b == '\r' && l.pos < len(l.src) && l.src[l.pos] == '\n') {
			// Unterminated — back up to not consume the newline.
			l.pos--
			return
		}
	}
}

// lexMultiLineLiteralString consumes content until closing ”'.
// Per TOML spec, up to 2 additional ' before ”' are content.
func (l *Lexer) lexMultiLineLiteralString() {
	quoteCount := 0
	quotesFound := false

	for l.pos < len(l.src) {
		b := l.src[l.pos]

		if quotesFound {
			if b != '\'' {
				return
			}
			quoteCount++
			l.pos++
			if quoteCount >= 5 {
				return
			}
			continue
		}

		l.pos++

		if b == '\'' {
			quoteCount++
			if quoteCount >= 3 {
				quotesFound = true
			}
		} else {
			quoteCount = 0
		}
	}
	// Unterminated — lexer continues.
}

// lexBareWord consumes a bare key, keyword (true/false/nan/inf), number,
// or datetime. These are distinguished by content after lexing.
// Also handles any unrecognized byte by consuming it as a single-byte BareKey token.
func (l *Lexer) lexBareWord(start int) Token {
	first := l.src[l.pos]

	// If starts with digit or sign (+/-), consume as numeric (allows . : + in content).
	if first >= '0' && first <= '9' || first == '+' || first == '-' {
		l.pos++ // consume first byte
		for l.pos < len(l.src) {
			if !isNumericChar(l.src[l.pos]) {
				break
			}
			l.pos++
		}
	} else if isBareWordChar(first) {
		// Bare key or keyword — no dots, no colons, no signs.
		for l.pos < len(l.src) {
			if !isBareWordChar(l.src[l.pos]) {
				break
			}
			l.pos++
		}
	} else {
		// Unrecognized byte — consume as a single-character token.
		// This ensures the lexer always makes forward progress.
		l.pos++
	}

	raw := l.src[start:l.pos]
	kind := classifyBareWord(raw)
	return Token{Kind: kind, Raw: raw, Offset: start}
}

// isBareWordChar returns true if the byte can be part of a bare word
// (key, number, datetime, bool, nan, inf).
// Note: '.' is NOT included — it's always a separate Dot token in key context.
// Numbers and datetimes that contain '.', ':', '+', 'T', 'Z' are handled by
// lexNumeric which is called when the first character is a digit or sign.
func isBareWordChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '-':
		return true
	default:
		return false
	}
}

// isNumericChar returns true if the byte can be part of a numeric value
// or datetime literal. This is a superset of bare key chars.
func isNumericChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '-' || b == '+' || b == '.' || b == ':':
		return true
	default:
		return false
	}
}

// classifyBareWord determines the token kind for a bare word.
func classifyBareWord(raw []byte) TokenKind {
	s := string(raw)

	// Bool
	if s == "true" || s == "false" {
		return Bool
	}

	// Special float values
	if s == "nan" || s == "inf" || s == "+nan" || s == "-nan" || s == "+inf" || s == "-inf" {
		return Float
	}

	// Empty (shouldn't happen but be safe)
	if len(raw) == 0 {
		return BareKey
	}

	first := raw[0]

	// Starts with digit or sign — could be number or datetime
	if first >= '0' && first <= '9' || first == '+' || first == '-' {
		return classifyNumericWord(raw)
	}

	// Everything else is a bare key
	return BareKey
}

// classifyNumericWord distinguishes between integers, floats, and datetimes
// for a word that starts with a digit or sign.
func classifyNumericWord(raw []byte) TokenKind {
	// Check for datetime markers: ':', 'T', 'Z' anywhere, or date pattern NNNN-NN-NN
	hasColon := false
	hasTorZ := false
	dashCount := 0

	for _, b := range raw {
		switch b {
		case ':':
			hasColon = true
		case 'T', 'Z':
			hasTorZ = true
		case '-':
			dashCount++
		default:
			// Other bytes don't affect datetime classification.
		}
	}

	// DateTime: has colon, or T/Z after digits, or date pattern (2+ dashes with 4+ leading digits)
	if hasColon || hasTorZ {
		return DateTime
	}
	// Date pattern: NNNN-NN-NN (exactly 2 dashes, length 10, digits in right positions)
	if dashCount >= 2 && len(raw) >= 10 {
		// Check if it looks like a date: first 4 chars are digits
		if len(raw) >= 4 && raw[0] >= '0' && raw[0] <= '9' && raw[3] >= '0' && raw[3] <= '9' && raw[4] == '-' {
			return DateTime
		}
	}

	// Hex/Oct/Bin prefixes
	if len(raw) > 2 && raw[0] == '0' {
		switch raw[1] {
		case 'x', 'X', 'o', 'O', 'b', 'B':
			return Integer
		default:
			// Not a prefixed integer literal.
		}
	}

	// Contains exponent → float
	for _, b := range raw {
		if b == 'e' || b == 'E' {
			return Float
		}
	}

	// Contains '.' → float (TOML dates use '-' not '.')
	if containsByte(string(raw), '.') {
		return Float
	}

	return Integer
}

// containsByte returns true if s contains byte b.
func containsByte(s string, b byte) bool {
	for i := range len(s) {
		if s[i] == b {
			return true
		}
	}
	return false
}
