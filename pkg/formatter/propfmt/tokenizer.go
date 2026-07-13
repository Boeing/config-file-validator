package propfmt

// TokenKind identifies the type of a properties file token.
type TokenKind int

const (
	// TokWhitespace is horizontal whitespace (spaces, tabs).
	TokWhitespace TokenKind = iota
	// TokNewline is a line ending (\n or \r\n).
	TokNewline
	// TokComment is a comment line (# or ! through end of line, not including newline).
	TokComment
	// TokKey is the key portion of a key-value pair (escape sequences preserved).
	TokKey
	// TokSeparator is the key-value separator (=, :, or first whitespace).
	TokSeparator
	// TokValue is the value portion (may include continuation lines).
	TokValue
)

// Token represents a single lexical token in a properties file.
type Token struct {
	Kind TokenKind
	Raw  []byte
}

// tokenize lexes properties source into a flat token stream.
// Every byte in src is accounted for in exactly one token.
// The structure of a properties file is line-based:
//   - Comment line: optional whitespace + (# or !) + content + newline
//   - Blank line: optional whitespace + newline
//   - Key-value line: key + separator + value + newline
//     (value may span multiple lines via \ continuation)
func tokenize(src []byte) []Token {
	var tokens []Token
	pos := 0

	for pos < len(src) {
		// Consume leading whitespace on this line (spaces, tabs, form feeds).
		wsStart := pos
		for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t' || src[pos] == '\f') {
			pos++
		}
		if pos > wsStart {
			tokens = append(tokens, Token{Kind: TokWhitespace, Raw: src[wsStart:pos]})
		}

		// End of input after whitespace.
		if pos >= len(src) {
			break
		}

		// Check what this line is.
		switch {
		case src[pos] == '\n':
			tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+1]})
			pos++

		case src[pos] == '\r' && pos+1 < len(src) && src[pos+1] == '\n':
			tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+2]})
			pos += 2

		case src[pos] == '\r':
			// Bare \r is a line terminator in Java properties.
			tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+1]})
			pos++

		case src[pos] == '#' || src[pos] == '!':
			// Comment line — consume through end of line.
			start := pos
			for pos < len(src) && src[pos] != '\n' && src[pos] != '\r' {
				pos++
			}
			tokens = append(tokens, Token{Kind: TokComment, Raw: src[start:pos]})

		default:
			// Key-value line.
			tokens = append(tokens, tokenizeKeyValue(src, &pos)...)
		}
	}

	return tokens
}

// tokenizeKeyValue lexes a key-value line starting at the key.
// Returns tokens for: key, separator, value (which may span continuation lines).
// Advances pos past the entire entry including continuation lines.
func tokenizeKeyValue(src []byte, pos *int) []Token {
	var tokens []Token

	// Lex key: characters until unescaped separator (=, :, or whitespace).
	keyStart := *pos
	for *pos < len(src) {
		b := src[*pos]
		if b == '\\' && *pos+1 < len(src) {
			*pos += 2 // skip escaped character
			continue
		}
		if b == '=' || b == ':' || b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' {
			break
		}
		*pos++
	}
	if *pos > keyStart {
		tokens = append(tokens, Token{Kind: TokKey, Raw: src[keyStart:*pos]})
	}

	// Lex separator: optional whitespace + (= or :) + optional whitespace,
	// OR just whitespace (space separator).
	sepStart := *pos
	// Skip whitespace before = or :
	for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
		*pos++
	}
	// Check for = or :
	if *pos < len(src) && (src[*pos] == '=' || src[*pos] == ':') {
		*pos++
		// Skip whitespace after = or :
		for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
			*pos++
		}
	}
	if *pos > sepStart {
		tokens = append(tokens, Token{Kind: TokSeparator, Raw: src[sepStart:*pos]})
	}

	// Lex value: everything until end of logical line.
	// A logical line continues if the physical line ends with \ (odd count of trailing backslashes).
	valueStart := *pos
	for {
		// Consume to end of physical line.
		for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
			*pos++
		}

		// Check for continuation.
		lineEnd := *pos
		if !endsWithOddBackslashes(src[valueStart:lineEnd]) {
			break // no continuation
		}

		// Consume the newline (part of the value via continuation).
		if *pos < len(src) {
			if src[*pos] == '\r' && *pos+1 < len(src) && src[*pos+1] == '\n' {
				*pos += 2
			} else if src[*pos] == '\n' || src[*pos] == '\r' {
				*pos++
			}
		}

		// Skip leading whitespace on continuation line (it's not part of the value
		// semantically, but we preserve it in the raw token for verbatim output).
	}

	if *pos > valueStart {
		tokens = append(tokens, Token{Kind: TokValue, Raw: src[valueStart:*pos]})
	}

	return tokens
}

// endsWithOddBackslashes returns true if the byte slice ends with an odd
// number of consecutive backslashes (indicating line continuation).
func endsWithOddBackslashes(b []byte) bool {
	count := 0
	for i := len(b) - 1; i >= 0 && b[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}
