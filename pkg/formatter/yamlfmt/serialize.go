// Serialization phase: converts the token stream back to bytes, stripping trailing whitespace.
package yamlfmt

import "bytes"

// serializeWithStrip walks tokens, strips trailing whitespace from each line,
// but emits TokBlockScalar tokens verbatim to preserve block scalar semantics
// (trailing spaces in content lines and trailing newlines for |+ chomping).
func serializeWithStrip(tokens []Token) []byte {
	var out []byte
	var line []byte

	// flushLineStripped trims trailing spaces/tabs and appends to out.
	flushLineStripped := func() {
		end := len(line)
		for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t') {
			end--
		}
		out = append(out, line[:end]...)
		line = line[:0]
	}

	// flushLineRaw appends accumulated line content to out without stripping.
	flushLineRaw := func() {
		out = append(out, line...)
		line = line[:0]
	}

	for _, tok := range tokens {
		if tok.Kind == TokBlockScalar {
			// Flush pending line content WITHOUT stripping — the trailing
			// space (e.g. from ": ") is needed before the block scalar header.
			flushLineRaw()
			// Emit block scalar raw, verbatim — no stripping.
			out = append(out, tok.Raw...)
			continue
		}

		// Accumulate into line buffer; flush on newlines.
		for _, b := range tok.Raw {
			if b == '\n' {
				// Check for CRLF: if line ends with \r, include it in the line
				// before stripping (strip only spaces/tabs).
				hasCR := len(line) > 0 && line[len(line)-1] == '\r'
				if hasCR {
					line = line[:len(line)-1]
				}
				flushLineStripped()
				if hasCR {
					out = append(out, '\r')
				}
				out = append(out, '\n')
			} else {
				line = append(line, b)
			}
		}
	}

	// Flush remaining content (strip trailing whitespace).
	flushLineStripped()
	return out
}

// endsWithBlockScalarPreservingNewlines checks whether the last meaningful token
// is a block scalar whose trailing newlines are semantically significant.
// Only |- (strip) allows removal. Both | (clip) and |+ (keep) preserve newlines.
func endsWithBlockScalarPreservingNewlines(tokens []Token) bool {
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i].Kind {
		case TokNewline, TokIndent, TokSpace:
			continue
		case TokBlockScalar:
			return !blockScalarHasStripChomping(tokens[i].Raw)
		default:
			return false
		}
	}
	return false
}

// blockScalarHasStripChomping checks if a block scalar header contains '-' (strip indicator).
// Only scans the indicator portion of the header (before any comment).
func blockScalarHasStripChomping(raw []byte) bool {
	nlIdx := bytes.IndexByte(raw, '\n')
	if nlIdx < 0 {
		return false
	}
	// Scan header characters before comment (space/tab + # starts comment).
	for _, b := range raw[:nlIdx] {
		if b == ' ' || b == '\t' || b == '#' {
			break
		}
		if b == '-' {
			return true
		}
	}
	return false
}
