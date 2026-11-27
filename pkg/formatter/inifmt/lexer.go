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
	// TokContinuation is an indented line continuing the previous value
	// (Python configparser style), minus the key line's own indentation.
	TokContinuation
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
//   - Continuation line: indented deeper than the key-value line above it
func tokenize(src []byte) []Token {
	var tokens []Token
	pos := 0
	// Indentation of the key-value line whose value is still open, or -1.
	keyIndent := -1

	for pos < len(src) {
		// Consume leading whitespace on this line (spaces, tabs).
		wsStart := pos
		for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t') {
			pos++
		}
		lineEnd := pos >= len(src) || src[pos] == '\n' || src[pos] == '\r'

		if keyIndent >= 0 && pos-wsStart > keyIndent && !lineEnd {
			if keyIndent > 0 {
				tokens = append(tokens, Token{Kind: TokWhitespace, Raw: src[wsStart : wsStart+keyIndent]})
			}
			start := wsStart + keyIndent
			for pos < len(src) && src[pos] != '\n' && src[pos] != '\r' {
				pos++
			}
			tokens = append(tokens, Token{Kind: TokContinuation, Raw: src[start:pos]})
			pos = consumeNewline(src, pos, &tokens)
			if string(tokens[len(tokens)-1].Raw) == "\r" {
				keyIndent = -1
			}
			continue
		}
		if lineEnd || src[pos] == '#' || src[pos] == ';' || src[pos] == '[' {
			keyIndent = -1
		} else {
			keyIndent = pos - wsStart
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
		if !lineEnd {
			pos = consumeNewline(src, pos, &tokens)
		}
		// ini.v1 only splits lines on LF, so a bare CR never starts a continuation.
		if n := len(tokens); n > 0 && tokens[n-1].Kind == TokNewline && string(tokens[n-1].Raw) == "\r" {
			keyIndent = -1
		}
	}

	return tokens
}

// consumeNewline appends the line ending at pos, if any, and returns the
// position after it.
func consumeNewline(src []byte, pos int, tokens *[]Token) int {
	switch {
	case pos+1 < len(src) && src[pos] == '\r' && src[pos+1] == '\n':
		*tokens = append(*tokens, Token{Kind: TokNewline, Raw: src[pos : pos+2]})
		return pos + 2
	case pos < len(src) && (src[pos] == '\n' || src[pos] == '\r'):
		*tokens = append(*tokens, Token{Kind: TokNewline, Raw: src[pos : pos+1]})
		return pos + 1
	}
	return pos
}

// tokenizeKeyValue lexes a key-value line starting at the key.
// Returns tokens for: key, separator, value.
// Advances pos past the entire entry (not including the trailing newline).
func tokenizeKeyValue(src []byte, pos *int) []Token {
	var tokens []Token

	// Lex key: characters until separator (=, :) or whitespace before separator.
	// INI keys do not support escape sequences — backslashes are literal.
	keyStart := *pos
	for *pos < len(src) {
		b := src[*pos]
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
	// Quoted values may contain CR/LF — scan to matching close quote first.
	valueStart := *pos
	if *pos < len(src) && (src[*pos] == '"' || src[*pos] == '\'') {
		// Quoted value — scan to closing quote. CR/LF inside are content.
		quote := src[*pos]
		*pos++ // skip opening quote
		for *pos < len(src) && src[*pos] != quote {
			*pos++
		}
		if *pos < len(src) {
			*pos++ // skip closing quote
		}
		// After closing quote, consume remainder to end of line.
		for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
			*pos++
		}
	} else {
		// Unquoted value — scan to line end.
		for *pos < len(src) && src[*pos] != '\n' && src[*pos] != '\r' {
			*pos++
		}
	}
	if *pos > valueStart {
		tokens = append(tokens, Token{Kind: TokValue, Raw: src[valueStart:*pos]})
	}

	return tokens
}

// withoutContinuations rebuilds the source with continuation lines left
// empty, so ini.v1 checks the keys while continuations stay part of their value.
func withoutContinuations(tokens []Token) []byte {
	var buf []byte
	for _, t := range tokens {
		if t.Kind != TokContinuation {
			buf = append(buf, t.Raw...)
		}
	}
	return buf
}
