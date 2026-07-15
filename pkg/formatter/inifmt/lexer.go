package inifmt

// TokenKind identifies the type of an INI file token.
type TokenKind int

const (
	// TokWhitespace is horizontal whitespace (spaces, tabs).
	TokWhitespace TokenKind = iota
	// TokNewline is a line ending (\n, \r\n, or bare \r).
	TokNewline
	// TokComment is a comment line (# or ; through end of line, not including newline).
	TokComment
	// TokSection is a section header ([name]).
	TokSection
	// TokKey is the key portion of a key-value pair.
	TokKey
	// TokSeparator is the key-value separator (= or :) with surrounding whitespace.
	TokSeparator
	// TokValue is the value portion of a key-value pair.
	TokValue
)

// Token represents a single lexical token in an INI file.
type Token struct {
	Kind TokenKind
	Raw  []byte
}

// tokenize lexes INI source into a flat token stream.
// Every byte in src is accounted for in exactly one token.
//
// INI file structure is line-based:
//   - Comment line: optional whitespace + (# or ;) + content + newline
//   - Blank line: optional whitespace + newline
//   - Section header: optional whitespace + [name] + newline
//   - Key-value line: key + separator + value + newline
func tokenize(src []byte) []Token {
	var tokens []Token
	pos := 0

	for pos < len(src) {
		// Consume leading whitespace on this line (spaces, tabs).
		wsStart := pos
		for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t') {
			pos++
		}
		if pos > wsStart {
			tokens = append(tokens, Token{Kind: TokWhitespace, Raw: src[wsStart:pos]})
		}

		// End of input after whitespace.
		if pos >= len(src) {
			break
		}

		// Determine line type by first non-whitespace character.
		switch src[pos] {
		case '\n':
			tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+1]})
			pos++

		case '\r':
			if pos+1 < len(src) && src[pos+1] == '\n' {
				tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+2]})
				pos += 2
			} else {
				tokens = append(tokens, Token{Kind: TokNewline, Raw: src[pos : pos+1]})
				pos++
			}

		case '#', ';':
			// Comment — consume through end of line (not including newline).
			start := pos
			for pos < len(src) && src[pos] != '\n' && src[pos] != '\r' {
				pos++
			}
			tokens = append(tokens, Token{Kind: TokComment, Raw: src[start:pos]})

		case '[':
			// Section header — consume to end of line (includes any trailing
			// content after the closing ], such as inline comments).
			start := pos
			for pos < len(src) && src[pos] != '\n' && src[pos] != '\r' {
				pos++
			}
			tokens = append(tokens, Token{Kind: TokSection, Raw: src[start:pos]})

		default:
			// Key-value line.
			tokens = append(tokens, tokenizeKeyValue(src, &pos)...)
		}
	}

	return tokens
}

// tokenizeKeyValue lexes a key-value line starting at the key.
// Returns tokens for: key, separator, value.
// Advances pos past the entire entry (not including the trailing newline).
func tokenizeKeyValue(src []byte, pos *int) []Token {
	var tokens []Token

	// Lex key: characters until unescaped separator (=, :) or whitespace before separator.
	keyStart := *pos
	for *pos < len(src) {
		b := src[*pos]
		if b == '\\' && *pos+1 < len(src) {
			*pos += 2 // skip escaped character
			continue
		}
		if b == '=' || b == ':' || b == '\n' || b == '\r' {
			break
		}
		// Whitespace might be before separator — lookahead.
		if b == ' ' || b == '\t' {
			// Look ahead past whitespace for a separator.
			ahead := *pos
			for ahead < len(src) && (src[ahead] == ' ' || src[ahead] == '\t') {
				ahead++
			}
			if ahead < len(src) && (src[ahead] == '=' || src[ahead] == ':') {
				break // whitespace is part of separator
			}
			// No separator found ahead — this whitespace is embedded in the key.
			// ini.v1 allows keys with spaces (e.g., "my key = val").
			// Include the whitespace in the key token and continue scanning.
			*pos++
			continue
		}
		*pos++
	}
	if *pos > keyStart {
		tokens = append(tokens, Token{Kind: TokKey, Raw: src[keyStart:*pos]})
	}

	// Lex separator: optional whitespace + (= or :) + optional whitespace.
	sepStart := *pos
	// Skip whitespace before separator character.
	for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
		*pos++
	}
	// Consume separator character.
	if *pos < len(src) && (src[*pos] == '=' || src[*pos] == ':') {
		*pos++
		// Skip whitespace after separator character.
		for *pos < len(src) && (src[*pos] == ' ' || src[*pos] == '\t') {
			*pos++
		}
	}
	if *pos > sepStart {
		tokens = append(tokens, Token{Kind: TokSeparator, Raw: src[sepStart:*pos]})
	}

	// Lex value: everything until end of line.
	valueStart := *pos
	for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
		*pos++
	}
	if *pos > valueStart {
		tokens = append(tokens, Token{Kind: TokValue, Raw: src[valueStart:*pos]})
	}

	return tokens
}
