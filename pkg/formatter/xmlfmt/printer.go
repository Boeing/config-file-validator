package xmlfmt

import (
	"bytes"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted applies formatting to the token stream.
func printFormatted(tokens []Token, opts formatter.Options, src []byte) []byte {
	if len(tokens) == 0 {
		return nil
	}

	indent := buildIndentString(opts)

	// Annotate tokens with depth and mixed-content info.
	annotate(tokens, src)

	// In "ignore" mode, insert formatting whitespace (newlines + indent).
	// In "preserve" mode, only modify existing indent tokens.
	if opts.XMLWhitespaceSensitivity == formatter.XMLWhitespaceIgnore {
		tokens = insertFormattingWhitespace(tokens, indent)
	} else {
		reindentExisting(tokens, indent)
	}

	// Apply self-closing space preference.
	applySelfClosingSpace(tokens, opts.XMLSelfClosingSpace)

	// Serialize.
	var buf bytes.Buffer
	for _, tok := range tokens {
		buf.Write(tok.Raw)
	}
	out := buf.Bytes()

	// Strip trailing whitespace from each line.
	out = stripTrailingWhitespace(out)

	// Final newline.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	return formatter.NormalizeLineEndings(out, opts.LineEnding)
}

// buildIndentString constructs the indent unit string from options.
func buildIndentString(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		width = 2
	}
	return strings.Repeat(" ", width)
}

// =============================================================================
// Annotation
// =============================================================================

// annotate sets Structural and Depth on TokIndent tokens based on tag nesting.
// Also detects mixed-content elements from the token stream.
func annotate(tokens []Token, _ []byte) {
	// Compute depth from tag nesting.
	depth := 0
	for i := range tokens {
		tokens[i].Depth = -1
		switch tokens[i].Kind {
		case TokIndent:
			tokens[i].Depth = depth
			tokens[i].Structural = true
		case TokOpenTag:
			depth++
		case TokCloseTag:
			if depth > 0 {
				depth--
			}
		default:
		}
	}

	// Detect mixed content and mark those indents as non-structural.
	detectAndMarkMixedContent(tokens)
}

// detectAndMarkMixedContent finds elements with both text and element children.
func detectAndMarkMixedContent(tokens []Token) {
	for i := range tokens {
		if tokens[i].Kind != TokOpenTag {
			continue
		}

		// Find matching close tag and check for mixed content.
		openDepth := tokens[i].Depth
		hasText := false
		hasChild := false

		for j := i + 1; j < len(tokens); j++ {
			switch tokens[j].Kind {
			case TokOpenTag, TokSelfClose:
				if tokens[j].Depth == openDepth+1 || (tokens[j].Kind == TokSelfClose && tokens[j].Depth == -1) {
					hasChild = true
				}
			case TokCloseTag:
				// Check if this is our matching close.
				// After a close tag, depth returns to openDepth-1.
				// We detect this by counting.
				d := 0
				for k := i; k <= j; k++ {
					switch tokens[k].Kind {
					case TokOpenTag:
						d++
					case TokCloseTag:
						d--
					default:
					}
				}
				if d == 0 {
					// Found matching close.
					if hasText && hasChild {
						// Mark all indents in this range as non-structural.
						for k := i + 1; k < j; k++ {
							if tokens[k].Kind == TokIndent {
								tokens[k].Structural = false
							}
						}
					}
					goto nextElement
				}
			case TokText:
				content := strings.TrimSpace(string(tokens[j].Raw))
				if content != "" {
					hasText = true
				}
			default:
			}
		}
	nextElement:
	}
}

// =============================================================================
// Ignore mode: insert formatting whitespace
// =============================================================================

// insertFormattingWhitespace restructures tokens for pretty-printed output.
// Removes whitespace-only text between tags, inserts proper newlines + indent.
func insertFormattingWhitespace(tokens []Token, indentUnit string) []Token {
	// First: remove whitespace-only text tokens between tags.
	cleaned := removeInsignificantWhitespace(tokens)

	// Second: insert newlines and indentation.
	var result []Token
	depth := 0

	for i, tok := range cleaned {
		switch tok.Kind {
		case TokOpenTag:
			// Newline + indent before open tag (except at depth 0, first element).
			if depth > 0 || (i > 0 && needsNewlineBefore(cleaned, i)) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)
			depth++

		case TokCloseTag:
			depth--
			// Newline + indent before close tag if previous was a tag (not text).
			if i > 0 && prevIsTag(cleaned, i) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)

		case TokSelfClose:
			if depth > 0 || (i > 0 && needsNewlineBefore(cleaned, i)) {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)

		case TokComment, TokProcInst, TokCDATA:
			if depth > 0 {
				result = appendNewlineIndent(result, depth, indentUnit)
			}
			result = append(result, tok)

		case TokXMLDecl, TokDoctype:
			result = append(result, tok)
			// Insert newline after declaration/doctype if more content follows.
			if i+1 < len(cleaned) {
				result = append(result, Token{Kind: TokNewline, Raw: []byte("\n")})
			}

		case TokText:
			// Keep text inline (no newline before it).
			result = append(result, tok)

		case TokIndent, TokNewline:
			// Skip old whitespace — we're inserting new.
			continue

		default:
			result = append(result, tok)
		}
	}

	return result
}

// removeInsignificantWhitespace removes TokText tokens that are whitespace-only
// (old indentation between tags), and all TokIndent/TokNewline tokens.
func removeInsignificantWhitespace(tokens []Token) []Token {
	var result []Token
	for _, tok := range tokens {
		switch tok.Kind {
		case TokIndent, TokNewline:
			// Remove old formatting whitespace.
			continue
		case TokText:
			// Keep only non-whitespace text.
			if strings.TrimSpace(string(tok.Raw)) != "" {
				result = append(result, tok)
			}
		default:
			result = append(result, tok)
		}
	}
	return result
}

// appendNewlineIndent appends a newline token and an indent token.
func appendNewlineIndent(tokens []Token, depth int, indentUnit string) []Token {
	tokens = append(tokens, Token{Kind: TokNewline, Raw: []byte("\n")})
	if depth > 0 {
		tokens = append(tokens, Token{Kind: TokIndent, Raw: []byte(strings.Repeat(indentUnit, depth))})
	}
	return tokens
}

// needsNewlineBefore returns true if a newline should be inserted before token at idx.
func needsNewlineBefore(tokens []Token, idx int) bool {
	if idx == 0 {
		return false
	}
	prev := tokens[idx-1]
	return prev.Kind == TokOpenTag || prev.Kind == TokCloseTag ||
		prev.Kind == TokSelfClose || prev.Kind == TokComment ||
		prev.Kind == TokXMLDecl || prev.Kind == TokDoctype || prev.Kind == TokProcInst
}

// prevIsTag returns true if the previous non-whitespace token is a tag.
func prevIsTag(tokens []Token, idx int) bool {
	for j := idx - 1; j >= 0; j-- {
		switch tokens[j].Kind {
		case TokIndent, TokNewline:
			continue
		case TokOpenTag, TokCloseTag, TokSelfClose, TokComment, TokCDATA, TokProcInst:
			return true
		default:
			return false
		}
	}
	return false
}

// =============================================================================
// Preserve mode: only modify existing indent
// =============================================================================

// reindentExisting modifies existing TokIndent tokens based on their depth.
// Does not insert or remove any tokens.
func reindentExisting(tokens []Token, indentUnit string) {
	for i := range tokens {
		if tokens[i].Kind != TokIndent || !tokens[i].Structural {
			continue
		}
		if tokens[i].Depth < 0 {
			continue
		}
		tokens[i].Raw = []byte(strings.Repeat(indentUnit, tokens[i].Depth))
	}
}

// =============================================================================
// Self-closing space
// =============================================================================

// applySelfClosingSpace ensures or removes space before /> in self-closing tags.
func applySelfClosingSpace(tokens []Token, wantSpace bool) {
	for i := range tokens {
		if tokens[i].Kind != TokSelfClose {
			continue
		}
		raw := tokens[i].Raw
		if len(raw) < 3 {
			continue
		}
		// Find the /> at the end.
		endsWithSlashGt := len(raw) >= 2 && raw[len(raw)-2] == '/' && raw[len(raw)-1] == '>'
		if !endsWithSlashGt {
			continue
		}
		hasSpace := len(raw) >= 3 && raw[len(raw)-3] == ' '

		if wantSpace && !hasSpace {
			// Insert space: <tag/> → <tag />
			newRaw := make([]byte, 0, len(raw)+1)
			newRaw = append(newRaw, raw[:len(raw)-2]...)
			newRaw = append(newRaw, ' ', '/', '>')
			tokens[i].Raw = newRaw
		} else if !wantSpace && hasSpace {
			// Remove space: <tag /> → <tag/>
			newRaw := make([]byte, 0, len(raw)-1)
			newRaw = append(newRaw, raw[:len(raw)-3]...)
			newRaw = append(newRaw, '/', '>')
			tokens[i].Raw = newRaw
		}
	}
}

// =============================================================================
// Utilities
// =============================================================================

func stripTrailingWhitespace(data []byte) []byte {
	var result []byte
	lineStart := 0
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			end := i
			for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
				end--
			}
			result = append(result, data[lineStart:end]...)
			result = append(result, '\n')
			lineStart = i + 1
		case '\r':
			end := i
			for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
				end--
			}
			result = append(result, data[lineStart:end]...)
			result = append(result, '\r')
			if i+1 < len(data) && data[i+1] == '\n' {
				result = append(result, '\n')
				i++
			}
			lineStart = i + 1
		default:
		}
	}
	if lineStart < len(data) {
		end := len(data)
		for end > lineStart && (data[end-1] == ' ' || data[end-1] == '\t') {
			end--
		}
		result = append(result, data[lineStart:end]...)
	}
	return result
}
