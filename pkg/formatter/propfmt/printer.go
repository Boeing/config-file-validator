package propfmt

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// entry represents a logical entry in a properties file.
type entry struct {
	comments    []Token // preceding comment lines (TokComment + TokNewline)
	key         Token   // the key token
	sep         Token   // the separator token
	value       Token   // the value token (may include continuations)
	newline     Token   // trailing newline
	blankBefore bool    // blank line preceded this entry
}

// printFormatted formats the token stream into canonical output.
func printFormatted(tokens []Token, opts formatter.Options) []byte {
	entries, trailingComments := groupEntries(tokens)

	if opts.SortKeys {
		slices.SortStableFunc(entries, func(a, b entry) int {
			aKey := decodeKey(a.key.Raw)
			bKey := decodeKey(b.key.Raw)
			return strings.Compare(aKey, bKey)
		})
	}

	var buf bytes.Buffer

	for i, e := range entries {
		// Insert blank line separator between groups if original had one.
		if i > 0 && e.blankBefore {
			buf.WriteByte('\n')
		}

		// Emit preceding comments.
		for _, c := range e.comments {
			buf.Write(c.Raw)
			if c.Kind == TokComment {
				buf.WriteByte('\n')
			}
		}

		// Emit key.
		buf.Write(e.key.Raw)

		// Emit normalized separator.
		buf.WriteString(" = ")

		// Emit value with continuations collapsed to canonical single-line form.
		if len(e.value.Raw) > 0 {
			buf.Write(decodeValueRaw(e.value.Raw))
		}

		buf.WriteByte('\n')
	}

	// Emit trailing comments.
	for _, c := range trailingComments {
		buf.Write(c.Raw)
		if c.Kind == TokComment {
			buf.WriteByte('\n')
		}
	}

	out := buf.Bytes()

	// Final newline handling.
	out = bytes.TrimRight(out, "\r\n")
	if opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, opts.LineEnding)
	return out
}

// groupEntries organizes tokens into logical entries with attached comments.
// Returns entries and any trailing comments.
func groupEntries(tokens []Token) ([]entry, []Token) {
	var entries []entry
	var pendingComments []Token
	var trailingComments []Token
	blankBefore := false

	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		switch tok.Kind {
		case TokNewline:
			// Blank line (newline without preceding key-value on this line).
			blankBefore = true
			i++

		case TokComment:
			// Collect comment.
			pendingComments = append(pendingComments, tok)
			i++
			// Consume following newline if present.
			if i < len(tokens) && tokens[i].Kind == TokNewline {
				i++
			}

		case TokKey:
			// Start of a key-value entry.
			e := entry{}
			e.comments = pendingComments
			e.blankBefore = blankBefore
			pendingComments = nil
			e.key = tok
			i++

			// Separator.
			if i < len(tokens) && tokens[i].Kind == TokSeparator {
				e.sep = tokens[i]
				i++
			}

			// Value.
			if i < len(tokens) && tokens[i].Kind == TokValue {
				e.value = tokens[i]
				i++
			}

			// Trailing newline.
			if i < len(tokens) && tokens[i].Kind == TokNewline {
				e.newline = tokens[i]
				i++
			}

			entries = append(entries, e)
			blankBefore = false

		default:
			// Whitespace or unexpected tokens — skip.
			i++
		}
	}

	// Any pending comments at the end are trailing.
	trailingComments = pendingComments

	return entries, trailingComments
}

// decodeKey strips escape sequences from a raw key token to produce
// the logical key string for sort comparison.
// Handles: \\= → =, \\: → :, \\\\ → \\, \\uXXXX → rune.
func decodeKey(raw []byte) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			if raw[i] == 'u' && i+4 < len(raw) {
				hex := string(raw[i+1 : i+5])
				if r, err := strconv.ParseUint(hex, 16, 32); err == nil {
					b.WriteRune(rune(r)) //nolint:gosec // r is bounded to 32 bits by ParseUint
					i += 4
					continue
				}
			}
			_ = b.WriteByte(raw[i])
		} else {
			_ = b.WriteByte(raw[i])
		}
	}
	return b.String()
}

// decodeValueRaw resolves continuation lines in a value's raw bytes.
// A continuation is \ + newline + optional leading whitespace on the next line.
// A trailing \ at EOF is also a continuation (to nothing) and is stripped.
// The result is the value content with continuations collapsed to nothing.
// Other escape sequences (\n, \t, \\, \uXXXX, etc.) are preserved verbatim.
func decodeValueRaw(raw []byte) []byte {
	var result []byte
	i := 0
	for i < len(raw) {
		if raw[i] == '\\' {
			if i+1 >= len(raw) {
				// Trailing backslash at EOF — continuation to nothing; drop it.
				break
			}
			next := raw[i+1]
			if next == '\n' {
				// Continuation: skip \ + LF + leading whitespace
				i += 2
				for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\f') {
					i++
				}
				continue
			}
			if next == '\r' {
				// Continuation: skip \ + CR + optional LF + leading whitespace
				i += 2
				if i < len(raw) && raw[i] == '\n' {
					i++
				}
				for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t' || raw[i] == '\f') {
					i++
				}
				continue
			}
			// Other escape sequence — preserve verbatim (e.g., \\, \n, \t, \uXXXX)
			result = append(result, raw[i], raw[i+1])
			i += 2
		} else {
			result = append(result, raw[i])
			i++
		}
	}
	return result
}
