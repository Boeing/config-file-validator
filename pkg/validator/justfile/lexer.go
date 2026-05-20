package gojust

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type lexer struct {
	src    []byte
	pos    int
	line   int
	col    int
	tokens []token

	// Indentation tracking for recipe bodies
	inRecipeBody        bool
	indentStack         []int
	atLineStart         bool
	recipeIndent        int
	lastLineHadColon    bool // tracks whether previous line contained a colon (recipe header)
	currentLineHasColon bool // tracks colon on the line being lexed
}

func newLexer(src []byte) *lexer {
	return &lexer{
		src:         src,
		line:        1,
		col:         1,
		indentStack: []int{0},
		atLineStart: true,
	}
}

func (l *lexer) lex() ([]token, error) {
	// Handle shebang line
	if l.pos+1 < len(l.src) && l.src[0] == '#' && l.src[1] == '!' {
		for l.pos < len(l.src) && l.src[l.pos] != '\n' {
			l.advance()
		}
		if l.pos < len(l.src) {
			l.advance() // skip newline
		}
	}

	for l.pos < len(l.src) {
		if l.inRecipeBody {
			if err := l.lexRecipeBody(); err != nil {
				return nil, err
			}
			continue
		}

		if l.atLineStart {
			// Check if this line is indented (recipe body start)
			if l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
				if l.lastLineHadColon {
					indent := l.measureIndent()
					l.indentStack = append(l.indentStack, 0) // base indent is 0 (non-recipe)
					l.recipeIndent = indent
					l.emit(tokenIndent, "")
					l.inRecipeBody = true
					// Don't skip spaces — lexRecipeBody will handle this line
					continue
				}
			}
			l.skipSpaces()
			if l.pos < len(l.src) && l.peek() == '\n' {
				l.emit(tokenNewline, "\n")
				l.advance()
				continue
			}
			if l.pos < len(l.src) && l.peek() == '#' {
				l.lexComment()
				continue
			}
		}
		l.atLineStart = false

		if l.pos >= len(l.src) {
			break
		}

		ch := l.peek()

		switch {
		case ch == '\n':
			l.emit(tokenNewline, "\n")
			l.advance()
			l.atLineStart = true
			l.lastLineHadColon = l.currentLineHasColon
			l.currentLineHasColon = false

		case ch == '\\' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == '\n' || (l.src[l.pos+1] == '\r' && l.pos+2 < len(l.src) && l.src[l.pos+2] == '\n')):
			l.lexLineContinuation()

		case ch == '#':
			l.lexComment()

		case ch == ' ' || ch == '\t' || ch == '\r':
			l.advance()

		case ch == ':' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			// := is assignment, not a recipe header colon — do NOT set currentLineHasColon
			l.emit(tokenAssign, ":=")
			l.advance()
			l.advance()

		case ch == ':':
			// Bare : indicates a recipe header
			l.emit(tokenColon, ":")
			l.advance()
			l.currentLineHasColon = true

		case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokenEquals, "==")
			l.advance()
			l.advance()

		case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '~':
			l.emit(tokenRegexMatch, "=~")
			l.advance()
			l.advance()

		case ch == '=':
			l.emit(tokenParamAssign, "=")
			l.advance()

		case ch == '!' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
			l.emit(tokenNotEquals, "!=")
			l.advance()
			l.advance()

		case ch == '!' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '~':
			l.emit(tokenRegexMismatch, "!~")
			l.advance()
			l.advance()

		case ch == '!':
			l.emit(tokenBang, "!")
			l.advance()

		case ch == '&' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '&':
			l.emit(tokenAnd, "&&")
			l.advance()
			l.advance()

		case ch == '|' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '|':
			l.emit(tokenOr, "||")
			l.advance()
			l.advance()

		case ch == '{' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{':
			l.emit(tokenInterpolStart, "{{")
			l.advance()
			l.advance()

		case ch == '}' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '}':
			l.emit(tokenInterpolEnd, "}}")
			l.advance()
			l.advance()

		case ch == '{':
			l.emit(tokenLBrace, "{")
			l.advance()

		case ch == '}':
			l.emit(tokenRBrace, "}")
			l.advance()

		case ch == '(':
			l.emit(tokenLParen, "(")
			l.advance()

		case ch == ')':
			l.emit(tokenRParen, ")")
			l.advance()

		case ch == '[':
			l.emit(tokenLBracket, "[")
			l.advance()

		case ch == ']':
			l.emit(tokenRBracket, "]")
			l.advance()

		case ch == '+':
			l.emit(tokenPlus, "+")
			l.advance()

		case ch == '/':
			l.emit(tokenSlash, "/")
			l.advance()

		case ch == '@' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '-':
			l.emit(tokenAtDash, "@-")
			l.advance()
			l.advance()

		case ch == '@':
			l.emit(tokenAt, "@")
			l.advance()

		case ch == '-' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '@':
			l.emit(tokenDashAt, "-@")
			l.advance()
			l.advance()

		case ch == '-':
			l.emit(tokenDash, "-")
			l.advance()

		case ch == '*':
			l.emit(tokenStar, "*")
			l.advance()

		case ch == ',':
			l.emit(tokenComma, ",")
			l.advance()

		case ch == '$':
			l.emit(tokenDollar, "$")
			l.advance()

		case ch == '?':
			l.emit(tokenQuestion, "?")
			l.advance()

		case ch == '"':
			if err := l.lexQuotedString(); err != nil {
				return nil, err
			}

		case ch == '\'':
			if err := l.lexRawString(); err != nil {
				return nil, err
			}

		case ch == '`':
			if err := l.lexBacktick(); err != nil {
				return nil, err
			}

		case isIdentStart(ch):
			// Check for string prefix before lexing as identifier.
			if handled, err := l.lexStringPrefix(); handled {
				if err != nil {
					return nil, err
				}
			} else {
				l.lexIdentifier()
			}

		default:
			return nil, l.errorf("unexpected character %q", ch)
		}
	}

	// Close any remaining indentation
	for len(l.indentStack) > 1 {
		l.emit(tokenDedent, "")
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
	}

	l.emit(tokenEOF, "")
	return l.tokens, nil
}

func (l *lexer) peek() byte {
	return l.src[l.pos]
}

func (l *lexer) advance() {
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *lexer) emit(typ tokenType, value string) {
	// Position is the start of the token, so capture before advancing.
	// For multi-char tokens, the caller should emit before advancing.
	l.tokens = append(l.tokens, token{
		Type:  typ,
		Value: value,
		Pos:   Position{Line: l.line, Column: l.col, Offset: l.pos},
	})
}

func (l *lexer) errorf(format string, args ...any) *ParseError {
	return &ParseError{
		Pos:     Position{Line: l.line, Column: l.col, Offset: l.pos},
		Message: fmt.Sprintf(format, args...),
	}
}

func (l *lexer) skipSpaces() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.advance()
	}
}

func (l *lexer) measureIndent() int {
	indent := 0
	pos := l.pos
	for pos < len(l.src) && (l.src[pos] == ' ' || l.src[pos] == '\t') {
		if l.src[pos] == '\t' {
			indent += 4
		} else {
			indent++
		}
		pos++
	}
	return indent
}

func (l *lexer) lexComment() {
	start := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	l.emit(tokenComment, string(l.src[start:l.pos]))
}

func (l *lexer) lexLineContinuation() {
	l.advance() // skip backslash
	if l.pos < len(l.src) && l.src[l.pos] == '\r' {
		l.advance()
	}
	l.advance() // skip newline
	l.skipSpaces()
}

func (l *lexer) lexIdentifier() {
	start := l.pos
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.advance()
	}
	word := string(l.src[start:l.pos])

	typ := tokenIdentifier
	switch word {
	case "alias":
		typ = tokenAlias
	case "export":
		typ = tokenExport
	case "unexport":
		typ = tokenUnexport
	case "import":
		typ = tokenImport
	case "mod":
		typ = tokenMod
	case "set":
		typ = tokenSet
	case "if":
		typ = tokenIf
	case "else":
		typ = tokenElse
	case "true":
		typ = tokenTrue
	case "false":
		typ = tokenFalse
	case "eager":
		typ = tokenEager
	default:
		// not a keyword, stays as tokenIdentifier
	}

	l.tokens = append(l.tokens, token{
		Type:  typ,
		Value: word,
		Pos:   Position{Line: l.line, Column: l.col - len(word), Offset: start},
	})
}

func (l *lexer) lexQuotedString() error {
	return l.lexQuotedStringAs(tokenString)
}

func (l *lexer) lexQuotedStringAs(typ tokenType) error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	// Check for indented string """
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
		return l.lexIndentedQuotedString(typ)
	}

	l.advance() // skip opening "
	var buf strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\\' {
			esc, err := l.lexEscapeSequence()
			if err != nil {
				return err
			}
			buf.WriteString(esc)
			continue
		}
		if ch == '"' {
			l.advance() // skip closing "
			l.tokens = append(l.tokens, token{Type: typ, Value: buf.String(), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(ch)
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated string"}
}

func (l *lexer) lexIndentedQuotedString(typ tokenType) error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	if typ == tokenString {
		typ = tokenIndentedString
	}

	l.advance() // "
	l.advance() // "
	l.advance() // "

	var buf strings.Builder
	for l.pos < len(l.src) {
		if l.src[l.pos] == '\\' {
			esc, err := l.lexEscapeSequence()
			if err != nil {
				return err
			}
			buf.WriteString(esc)
			continue
		}
		if l.src[l.pos] == '"' && l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
			l.advance() // "
			l.advance() // "
			l.advance() // "
			l.tokens = append(l.tokens, token{Type: typ, Value: dedentString(buf.String()), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(l.src[l.pos])
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated indented string"}
}

func (l *lexer) lexRawString() error {
	return l.lexRawStringAs(tokenRawString)
}

func (l *lexer) lexRawStringAs(typ tokenType) error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	// Check for indented raw string '''
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == '\'' && l.src[l.pos+2] == '\'' {
		return l.lexIndentedRawString(typ)
	}

	l.advance() // skip opening '
	var buf strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\'' {
			l.advance() // skip closing '
			l.tokens = append(l.tokens, token{Type: typ, Value: buf.String(), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(ch)
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated raw string"}
}

func (l *lexer) lexIndentedRawString(typ tokenType) error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	if typ == tokenRawString {
		typ = tokenIndentedRawString
	}

	l.advance() // '
	l.advance() // '
	l.advance() // '

	var buf strings.Builder
	for l.pos < len(l.src) {
		if l.src[l.pos] == '\'' && l.pos+2 < len(l.src) && l.src[l.pos+1] == '\'' && l.src[l.pos+2] == '\'' {
			l.advance() // '
			l.advance() // '
			l.advance() // '
			l.tokens = append(l.tokens, token{Type: typ, Value: dedentString(buf.String()), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(l.src[l.pos])
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated indented raw string"}
}

func (l *lexer) lexBacktick() error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	// Check for indented backtick ```
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == '`' && l.src[l.pos+2] == '`' {
		return l.lexIndentedBacktick()
	}

	l.advance() // skip opening `
	var buf strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '`' {
			l.advance()
			l.tokens = append(l.tokens, token{Type: tokenBacktick, Value: buf.String(), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(ch)
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated backtick"}
}

func (l *lexer) lexIndentedBacktick() error {
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	l.advance() // `
	l.advance() // `
	l.advance() // `

	var buf strings.Builder
	for l.pos < len(l.src) {
		if l.src[l.pos] == '`' && l.pos+2 < len(l.src) && l.src[l.pos+1] == '`' && l.src[l.pos+2] == '`' {
			l.advance() // `
			l.advance() // `
			l.advance() // `
			l.tokens = append(l.tokens, token{Type: tokenIndentedBacktick, Value: dedentString(buf.String()), Pos: startPos})
			return nil
		}
		_ = buf.WriteByte(l.src[l.pos])
		l.advance()
	}
	return &ParseError{Pos: startPos, Message: "unterminated indented backtick"}
}

func (l *lexer) lexEscapeSequence() (string, error) {
	l.advance() // skip backslash
	if l.pos >= len(l.src) {
		return "", l.errorf("unterminated escape sequence")
	}
	ch := l.src[l.pos]
	l.advance()
	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 't':
		return "\t", nil
	case '"':
		return "\"", nil
	case '\\':
		return "\\", nil
	case '\n':
		return "", nil // line continuation
	case '\r':
		if l.pos < len(l.src) && l.src[l.pos] == '\n' {
			l.advance()
		}
		return "", nil
	case 'u':
		return l.lexUnicodeEscape()
	default:
		return "", l.errorf("unknown escape sequence '\\%c'", ch)
	}
}

func (l *lexer) lexUnicodeEscape() (string, error) {
	if l.pos >= len(l.src) || l.src[l.pos] != '{' {
		return "", l.errorf("expected '{' after '\\u'")
	}
	l.advance() // skip {

	start := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != '}' {
		ch := l.src[l.pos]
		if !isHexDigit(ch) {
			return "", l.errorf("invalid character %q in unicode escape", ch)
		}
		l.advance()
	}
	if l.pos >= len(l.src) {
		return "", l.errorf("unterminated unicode escape")
	}

	hex := string(l.src[start:l.pos])
	l.advance() // skip }

	var codepoint int
	for _, ch := range hex {
		codepoint = codepoint*16 + hexValRune(ch)
	}
	if !utf8.ValidRune(rune(codepoint)) {
		return "", l.errorf("invalid unicode codepoint U+%s", strings.ToUpper(hex))
	}
	return string(rune(codepoint)), nil
}

// lexRecipeBody lexes lines inside a recipe body, handling indentation
// and interpolation.
func (l *lexer) lexRecipeBody() error {
	// Measure indent without consuming
	indent := l.measureIndent()

	// Empty line — consume whitespace and newline, stay in recipe body
	savePos, saveLine, saveCol := l.pos, l.line, l.col
	l.skipSpaces()
	if l.pos >= len(l.src) || l.src[l.pos] == '\n' {
		if l.pos < len(l.src) {
			l.emit(tokenNewline, "\n")
			l.advance()
		}
		return nil
	}

	// Dedent — line is not indented enough, exit recipe body
	if indent < l.recipeIndent {
		// Restore position to start of line so main loop can re-lex
		l.pos, l.line, l.col = savePos, saveLine, saveCol
		l.inRecipeBody = false
		l.emit(tokenDedent, "")
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.atLineStart = true
		return nil
	}

	// Restore to start of line, then skip exactly the recipe's base indentation.
	// Any indentation beyond that is preserved as part of the line text.
	l.pos, l.line, l.col = savePos, saveLine, saveCol
	l.skipIndent(l.recipeIndent)

	// Check for recipe line prefix
	if l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '@' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '-' {
			l.emit(tokenAtDash, "@-")
			l.advance()
			l.advance()
		} else if ch == '-' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '@' {
			l.emit(tokenDashAt, "-@")
			l.advance()
			l.advance()
		} else if ch == '@' {
			l.emit(tokenAt, "@")
			l.advance()
		} else if ch == '-' {
			l.emit(tokenDash, "-")
			l.advance()
		}
	}

	// Lex the rest of the line as text with interpolations
	return l.lexRecipeLine()
}

// skipIndent skips exactly n units of indentation (spaces=1, tabs=4).
func (l *lexer) skipIndent(n int) {
	skipped := 0
	for l.pos < len(l.src) && skipped < n {
		switch ch := l.src[l.pos]; ch {
		case '\t':
			skipped += 4
		case ' ':
			skipped++
		default:
			return
		}
		l.advance()
	}
}

func (l *lexer) lexRecipeLine() error {
	var buf strings.Builder
	startPos := Position{Line: l.line, Column: l.col, Offset: l.pos}

	flushText := func() {
		if buf.Len() > 0 {
			l.tokens = append(l.tokens, token{Type: tokenText, Value: buf.String(), Pos: startPos})
			buf.Reset()
			startPos = Position{Line: l.line, Column: l.col, Offset: l.pos}
		}
	}

	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		// Line continuation: backslash followed by newline joins with next line
		if ch == '\\' && l.pos+1 < len(l.src) {
			next := l.src[l.pos+1]
			if next == '\n' || (next == '\r' && l.pos+2 < len(l.src) && l.src[l.pos+2] == '\n') {
				_ = buf.WriteByte('\\')
				l.advance() // skip backslash
				if l.src[l.pos] == '\r' {
					l.advance()
				}
				l.advance() // skip newline
				// Flush current text, emit newline, then start new line
				flushText()
				l.emit(tokenNewline, "\n")
				// The continuation line is a new recipe line; return so
				// lexRecipeBody handles its indentation properly
				return nil
			}
		}

		if ch == '\n' {
			break
		}

		if ch == '{' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{' {
			// {{{{ is an escape for literal {{ in text
			if l.pos+3 < len(l.src) && l.src[l.pos+2] == '{' && l.src[l.pos+3] == '{' {
				buf.WriteString("{{")
				l.advance()
				l.advance()
				l.advance()
				l.advance()
				continue
			}
			flushText()
			l.emit(tokenInterpolStart, "{{")
			l.advance()
			l.advance()

			// Lex the expression inside the interpolation
			depth := 1
			for l.pos < len(l.src) && depth > 0 {
				if l.src[l.pos] == '{' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '{' {
					depth++
					l.emit(tokenInterpolStart, "{{")
					l.advance()
					l.advance()
					continue
				}
				if l.src[l.pos] == '}' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '}' {
					depth--
					if depth == 0 {
						l.emit(tokenInterpolEnd, "}}")
						l.advance()
						l.advance()
						break
					}
					l.emit(tokenInterpolEnd, "}}")
					l.advance()
					l.advance()
					continue
				}

				if err := l.lexExpressionToken(); err != nil {
					return err
				}
			}
			startPos = Position{Line: l.line, Column: l.col, Offset: l.pos}
			continue
		}
		_ = buf.WriteByte(l.src[l.pos])
		l.advance()
	}

	flushText()

	if l.pos < len(l.src) {
		l.emit(tokenNewline, "\n")
		l.advance()
	}
	return nil
}

// stringPrefixes maps single-character prefixes to their token types for
// quoted and raw string variants. To add a new string prefix (e.g. g"..."),
// add an entry here — no other lexer changes needed.
var stringPrefixes = map[byte]struct {
	quoted tokenType
	raw    tokenType
}{
	'f': {tokenFormatString, tokenFormatString},
	'x': {tokenShellExpandedString, tokenShellExpandedRawString},
}

// lexStringPrefix checks if the current character is a string prefix (f, x, etc.)
// followed by a quote. Returns (true, err) if it handled the token, (false, nil)
// if the character is not a string prefix and should be lexed as an identifier.
func (l *lexer) lexStringPrefix() (bool, error) {
	ch := l.src[l.pos]
	prefix, ok := stringPrefixes[ch]
	if !ok || l.pos+1 >= len(l.src) {
		return false, nil
	}
	next := l.src[l.pos+1]
	switch next {
	case '"':
		l.advance() // skip prefix char
		return true, l.lexQuotedStringAs(prefix.quoted)
	case '\'':
		l.advance() // skip prefix char
		return true, l.lexRawStringAs(prefix.raw)
	}
	return false, nil
}

// lexExpressionToken lexes a single token that can appear inside an expression.
// Used by both the main lexer and the interpolation lexer inside recipe bodies.
func (l *lexer) lexExpressionToken() error {
	ch := l.src[l.pos]
	switch {
	case ch == ' ' || ch == '\t':
		l.advance()
	case ch == '"':
		return l.lexQuotedString()
	case ch == '\'':
		return l.lexRawString()
	case ch == '`':
		return l.lexBacktick()
	case ch == '+':
		l.emit(tokenPlus, "+")
		l.advance()
	case ch == '/':
		l.emit(tokenSlash, "/")
		l.advance()
	case ch == '(':
		l.emit(tokenLParen, "(")
		l.advance()
	case ch == ')':
		l.emit(tokenRParen, ")")
		l.advance()
	case ch == ',':
		l.emit(tokenComma, ",")
		l.advance()
	case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
		l.emit(tokenEquals, "==")
		l.advance()
		l.advance()
	case ch == '=' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '~':
		l.emit(tokenRegexMatch, "=~")
		l.advance()
		l.advance()
	case ch == '!' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '=':
		l.emit(tokenNotEquals, "!=")
		l.advance()
		l.advance()
	case ch == '!' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '~':
		l.emit(tokenRegexMismatch, "!~")
		l.advance()
		l.advance()
	case ch == '&' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '&':
		l.emit(tokenAnd, "&&")
		l.advance()
		l.advance()
	case ch == '|' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '|':
		l.emit(tokenOr, "||")
		l.advance()
		l.advance()
	case ch == '{':
		l.emit(tokenLBrace, "{")
		l.advance()
	case ch == '}':
		l.emit(tokenRBrace, "}")
		l.advance()
	case isIdentStart(ch):
		l.lexIdentifier()
	default:
		return l.errorf("unexpected character %q in interpolation", ch)
	}
	return nil
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '-'
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func hexVal(ch byte) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10
	}
	return 0
}

func hexValRune(ch rune) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10
	}
	return 0
}

// dedentString implements just's indented string dedent algorithm:
// 1. Strip the leading newline (if present)
// 2. Find the common leading whitespace prefix (by character count) of all non-empty lines
// 3. Strip that prefix from every line; whitespace-only lines become empty
func dedentString(s string) string {
	if len(s) > 0 && s[0] == '\n' {
		s = s[1:]
	} else if len(s) > 1 && s[0] == '\r' && s[1] == '\n' {
		s = s[2:]
	}

	lines := strings.Split(s, "\n")

	// Find minimum indentation (character count) of non-empty lines.
	// Tabs count as one character, matching just's dedent behavior.
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := 0
		for _, ch := range line {
			if ch != ' ' && ch != '\t' {
				break
			}
			indent++
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			_ = b.WriteByte('\n')
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) >= minIndent {
			b.WriteString(line[minIndent:])
		} else {
			b.WriteString(line)
		}
	}
	return b.String()
}
