package tomlfmt

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// PrintOptions holds configuration for the printer.
type PrintOptions struct {
	Indent        string
	ColumnWidth   int
	TrailingComma bool
	AllowedBlanks int
	SortKeys      bool
	FinalNewline  bool
	LineEnding    formatter.LineEnding
}

// DefaultPrintOptions returns the default print options matching taplo defaults.
func DefaultPrintOptions() PrintOptions {
	return PrintOptions{
		Indent:        "  ",
		ColumnWidth:   80,
		TrailingComma: true,
		AllowedBlanks: 2,
		SortKeys:      false,
		FinalNewline:  true,
		LineEnding:    formatter.LineEndingLF,
	}
}

// Printer formats TOML groups into canonical output.
type Printer struct {
	opts PrintOptions
	buf  bytes.Buffer
}

// NewPrinter creates a Printer with the given options.
func NewPrinter(opts PrintOptions) *Printer {
	return &Printer{opts: opts}
}

// Print formats the groups and returns the formatted output.
func (p *Printer) Print(groups []Group) []byte {
	p.buf.Reset()

	if p.opts.SortKeys {
		groups = p.sortGroups(groups)
	}

	inTable := false
	started := false // tracks whether we've emitted any non-blank content
	for i, group := range groups {
		switch group.Kind {
		case GroupBlank:
			if !started {
				continue // Skip leading blank lines.
			}
			// Count actual newlines in this blank region = number of blank lines.
			nlCount := 0
			for _, tok := range group.Tokens {
				if tok.Kind == Newline {
					nlCount++
				}
			}
			// Emit up to AllowedBlanks blank lines.
			emit := nlCount
			if emit > p.opts.AllowedBlanks {
				emit = p.opts.AllowedBlanks
			}
			if emit < 1 {
				emit = 1
			}
			for range emit {
				p.writeNewline()
			}

		case GroupComment:
			started = true
			commentIndent := ""
			if inTable && p.opts.Indent != "" {
				commentIndent = p.opts.Indent
			}
			p.printComment(group, commentIndent)

		case GroupTable, GroupArrayTable:
			inTable = true
			started = true
			// Ensure blank line before table headers (except at start,
			// and except when preceded by a comment which already provides
			// visual separation).
			if i > 0 && groups[i-1].Kind != GroupComment {
				p.ensureBlankLine()
			}
			p.printTableHeader(group)

		case GroupEntry:
			started = true
			entryDepth := 0
			if inTable && p.opts.Indent != "" {
				p.buf.WriteString(p.opts.Indent)
				entryDepth = 1
			}
			p.printEntry(group, entryDepth)
		default:
			// Unknown group kind — skip.
		}
	}

	out := p.buf.Bytes()

	// Trim trailing whitespace/newlines and apply FinalNewline.
	out = bytes.TrimRight(out, "\r\n")
	if p.opts.FinalNewline {
		out = append(out, '\n')
	}

	out = formatter.NormalizeLineEndings(out, p.opts.LineEnding)

	return out
}

// printComment writes comment lines with preserved content.
func (p *Printer) printComment(group Group, indent string) {
	for _, tok := range group.Tokens {
		switch tok.Kind {
		case Comment:
			p.buf.WriteString(indent)
			p.buf.Write(tok.Raw)
		case Newline:
			p.writeNewline()
		default:
			// Whitespace and other token kinds — don't emit (we control indentation).
		}
	}
}

// printTableHeader writes a table or array table header.
func (p *Printer) printTableHeader(group Group) {
	// Skip leading whitespace, emit everything else.
	// Preserve whitespace before inline comments (e.g., [table] # comment).
	started := false
	for i, tok := range group.Tokens {
		switch tok.Kind {
		case Whitespace:
			if !started {
				continue // Skip leading whitespace.
			}
			// Preserve whitespace if followed by a comment.
			if i+1 < len(group.Tokens) && group.Tokens[i+1].Kind == Comment {
				p.buf.WriteByte(' ') // Normalize to single space before comment.
			}
		case Newline:
			p.writeNewline()
		default:
			started = true
			p.buf.Write(tok.Raw)
		}
	}
}

// printEntry writes a key = value entry with normalized spacing.
// depth is the current indentation depth (0 for top-level, 1 for inside a table).
func (p *Printer) printEntry(group Group, depth int) {
	tokens := group.Tokens

	// Split entry tokens into: key tokens, equals, value tokens, trailing comment, newline.
	keyEnd, equalsIdx, valueStart, valueEnd, commentStart := splitEntry(tokens)

	// Emit key.
	for i := 0; i <= keyEnd; i++ {
		tok := tokens[i]
		if tok.Kind == Whitespace {
			continue // Skip whitespace in key (between dotted segments there shouldn't be ws anyway)
		}
		p.buf.Write(tok.Raw)
	}

	// Emit normalized separator.
	_ = equalsIdx
	p.buf.WriteString(" = ")

	// Emit value.
	// Calculate prefix length for column width check (indent + key + " = ").
	keyLen := 0
	for i := 0; i <= keyEnd; i++ {
		if tokens[i].Kind != Whitespace {
			keyLen += len(tokens[i].Raw)
		}
	}
	prefixLen := len(p.opts.Indent) + keyLen + 3 // 3 for " = "
	p.printValue(tokens[valueStart:valueEnd+1], depth, prefixLen)

	// Emit trailing comment if present.
	if commentStart >= 0 {
		p.buf.WriteString(" ")
		for i := commentStart; i < len(tokens); i++ {
			tok := tokens[i]
			if tok.Kind == Comment {
				p.buf.Write(tok.Raw)
			}
		}
	}

	p.writeNewline()
}

// printValue writes value tokens. For simple values, emit verbatim.
// For arrays and inline tables, normalize internal spacing.
func (p *Printer) printValue(tokens []Token, depth int, prefixLen int) {
	if len(tokens) == 0 {
		return
	}

	// If value contains comments, emit verbatim — normalizing around
	// comments requires complex logic for each container type.
	for _, tok := range tokens {
		if tok.Kind == Comment {
			for _, t := range tokens {
				p.buf.Write(t.Raw)
			}
			return
		}
	}

	first := tokens[0]

	switch first.Kind {
	case BracketOpen:
		p.printArray(tokens, depth, prefixLen)
	case BraceOpen:
		p.printInlineTable(tokens, depth)
	default:
		// Scalar value or multiline string — emit verbatim.
		for _, tok := range tokens {
			p.buf.Write(tok.Raw)
		}
	}
}

// printArray formats an array value. Applies auto-expand/collapse and
// trailing comma normalization.
func (p *Printer) printArray(tokens []Token, depth int, prefixLen int) {
	// Split array into elements.
	elements := splitArrayElements(tokens)

	// Check if it contains comments (force multiline).
	hasComments := false
	for _, tok := range tokens {
		if tok.Kind == Comment {
			hasComments = true
			break
		}
	}

	// Calculate single-line length.
	singleLineLen := estimateSingleLineArray(elements)

	// Decision: multiline or single-line?
	// - Has comments → always multiline (can't collapse comments into one line)
	// - Exceeds column width (including key prefix) → multiline
	// - Fits and was originally multiline → collapse (taplo array_auto_collapse default)
	// - Fits and was originally inline → stay inline
	multiline := hasComments || (prefixLen+singleLineLen) > p.opts.ColumnWidth

	if multiline {
		p.printArrayMultiline(elements, depth)
	} else {
		p.printArrayInline(elements)
	}
}

// printArrayInline writes an array on a single line: [elem, elem, elem]
func (p *Printer) printArrayInline(elements [][]Token) {
	p.buf.WriteByte('[')
	for i, elem := range elements {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.writeValueTokensTrimmed(elem)
	}
	p.buf.WriteByte(']')
}

// printArrayMultiline writes an array with one element per line.
// Preserves comments between elements.
func (p *Printer) printArrayMultiline(elements [][]Token, depth int) {
	// For value internals (arrays, inline tables), always use at least 2 spaces
	// for indentation even if the table-level indent is empty. This matches
	// taplo's behavior where indent_string applies to value formatting
	// independently of indent_entries.
	valueIndent := p.opts.Indent
	if valueIndent == "" {
		valueIndent = "  "
	}
	elemIndent := strings.Repeat(valueIndent, depth+1)
	closeIndent := strings.Repeat(valueIndent, depth)

	p.buf.WriteByte('[')
	for _, elem := range elements {
		p.writeNewline()
		// Separate comments from value tokens to avoid duplication.
		// Comments are emitted on their own lines above the value.
		var comments []Token
		var valueTokens []Token
		for _, tok := range elem {
			switch tok.Kind {
			case Whitespace, Newline:
				continue
			case Comment:
				comments = append(comments, tok)
			default:
				valueTokens = append(valueTokens, tok)
			}
		}
		// Emit leading comments.
		for _, c := range comments {
			p.buf.WriteString(elemIndent)
			p.buf.Write(c.Raw)
			p.writeNewline()
		}
		// Emit value.
		if len(valueTokens) > 0 {
			p.buf.WriteString(elemIndent)
			p.writeValueTokensTrimmed(valueTokens)
			p.buf.WriteByte(',')
		}
	}
	p.writeNewline()
	p.buf.WriteString(closeIndent)
	p.buf.WriteByte(']')
}

// printInlineTable normalizes spacing inside an inline table.
// Produces: { key = val, key2 = val2 }
func (p *Printer) printInlineTable(tokens []Token, depth int) {
	// Extract key-value pairs from the inline table tokens.
	// Skip opening { and closing }.
	inner := tokens[1:] // skip {
	// Find closing }
	closeIdx := -1
	braceDepth := 1
	for i, tok := range inner {
		switch tok.Kind {
		case BraceOpen:
			braceDepth++
		case BraceClose:
			braceDepth--
			if braceDepth == 0 {
				closeIdx = i
			}
		default:
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		// Malformed — emit raw.
		for _, tok := range tokens {
			p.buf.Write(tok.Raw)
		}
		return
	}

	content := inner[:closeIdx]

	// Split into key-value pairs by comma (at depth 0).
	pairs := splitByComma(content)

	// Empty inline table.
	if len(pairs) == 0 {
		p.buf.WriteString("{}")
		return
	}

	// Emit as single-line: { key = val, key2 = val2 }
	p.buf.WriteString("{ ")
	for i, pair := range pairs {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.writeInlineTablePair(pair, depth)
	}
	p.buf.WriteString(" }")
}

// writeInlineTablePair writes a single key = value pair in an inline table
// with normalized spacing.
func (p *Printer) writeInlineTablePair(tokens []Token, depth int) {
	// Find equals.
	eqIdx := -1
	for i, tok := range tokens {
		if tok.Kind == Equals {
			eqIdx = i
			break
		}
	}
	if eqIdx < 0 {
		// Malformed — emit raw.
		for _, tok := range tokens {
			p.buf.Write(tok.Raw)
		}
		return
	}

	// Emit key (skip whitespace).
	for i := 0; i < eqIdx; i++ {
		if tokens[i].Kind != Whitespace {
			p.buf.Write(tokens[i].Raw)
		}
	}

	p.buf.WriteString(" = ")

	// Emit value (skip leading whitespace, recurse for nested structures).
	valueTokens := trimLeadingWhitespace(tokens[eqIdx+1:])
	valueTokens = trimTrailingWhitespace(valueTokens)
	p.printValue(valueTokens, depth+1, 0)
}

// writeValueTokensTrimmed writes value tokens with leading/trailing whitespace removed.
func (p *Printer) writeValueTokensTrimmed(tokens []Token) {
	trimmed := trimLeadingWhitespace(tokens)
	trimmed = trimTrailingWhitespace(trimmed)
	for _, tok := range trimmed {
		if tok.Kind == Newline || tok.Kind == Whitespace {
			continue
		}
		p.buf.Write(tok.Raw)
	}
}

// splitArrayElements splits array tokens into individual element token slices.
// Splits on comma at depth 0, skipping the outer [ and ].
func splitArrayElements(tokens []Token) [][]Token {
	if len(tokens) < 2 {
		return nil
	}

	// Skip opening [ and find closing ].
	inner := tokens[1:]
	closeIdx := -1
	depth := 1
	for i, tok := range inner {
		switch tok.Kind {
		case BracketOpen:
			depth++
		case BracketClose:
			depth--
			if depth == 0 {
				closeIdx = i
			}
		default:
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		closeIdx = len(inner)
	}
	content := inner[:closeIdx]

	return splitByComma(content)
}

// splitByComma splits tokens into groups separated by commas at depth 0.
// Handles nested arrays and inline tables.
func splitByComma(tokens []Token) [][]Token {
	var result [][]Token
	var current []Token
	depth := 0

	for _, tok := range tokens {
		switch tok.Kind {
		case BracketOpen, BraceOpen:
			depth++
			current = append(current, tok)
		case BracketClose, BraceClose:
			depth--
			current = append(current, tok)
		case Comma:
			if depth == 0 {
				if hasNonWhitespace(current) {
					result = append(result, current)
				}
				current = nil
			} else {
				current = append(current, tok)
			}
		default:
			current = append(current, tok)
		}
	}
	if hasNonWhitespace(current) {
		result = append(result, current)
	}
	return result
}

// estimateSingleLineArray estimates the character length if the array
// were written on a single line.
func estimateSingleLineArray(elements [][]Token) int {
	length := 2 // [ and ]
	for i, elem := range elements {
		if i > 0 {
			length += 2 // ", "
		}
		for _, tok := range elem {
			if tok.Kind != Whitespace && tok.Kind != Newline && tok.Kind != Comment {
				length += len(tok.Raw)
			}
		}
	}
	return length
}

// hasNonWhitespace returns true if the token slice contains any
// non-whitespace, non-newline token (comments count as content).
func hasNonWhitespace(tokens []Token) bool {
	for _, tok := range tokens {
		if tok.Kind != Whitespace && tok.Kind != Newline {
			return true
		}
	}
	return false
}

// trimLeadingWhitespace removes leading Whitespace and Newline tokens.
func trimLeadingWhitespace(tokens []Token) []Token {
	for len(tokens) > 0 && (tokens[0].Kind == Whitespace || tokens[0].Kind == Newline) {
		tokens = tokens[1:]
	}
	return tokens
}

// trimTrailingWhitespace removes trailing Whitespace and Newline tokens.
func trimTrailingWhitespace(tokens []Token) []Token {
	for len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if last.Kind != Whitespace && last.Kind != Newline {
			break
		}
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

// splitEntry identifies the structural parts of an entry's token slice.
// Returns indices: keyEnd, equalsIdx, valueStart, valueEnd, commentStart.
// commentStart is -1 if no trailing comment.
func splitEntry(tokens []Token) (keyEnd, equalsIdx, valueStart, valueEnd, commentStart int) {
	equalsIdx = -1
	commentStart = -1
	keyEnd = -1
	valueStart = -1

	// Find the equals sign.
	for i, tok := range tokens {
		if tok.Kind == Equals {
			equalsIdx = i
			break
		}
		if tok.Kind != Whitespace {
			keyEnd = i
		}
	}

	if equalsIdx < 0 {
		// No equals found — malformed, emit everything as key.
		return len(tokens) - 1, -1, -1, -1, -1
	}

	// Find value start (first non-whitespace after equals).
	for i := equalsIdx + 1; i < len(tokens); i++ {
		if tokens[i].Kind != Whitespace {
			valueStart = i
			break
		}
	}

	if valueStart < 0 {
		return keyEnd, equalsIdx, -1, -1, -1
	}

	// Find value end and trailing comment.
	// Value ends before trailing comment or newline.
	// Track bracket depth to handle multiline values.
	depth := 0
	for i := valueStart; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.Kind {
		case BracketOpen, BraceOpen:
			depth++
		case BracketClose, BraceClose:
			depth--
		case Comment:
			if depth == 0 {
				commentStart = i
				// Value ends at last non-whitespace before comment.
				for valueEnd = i - 1; valueEnd >= valueStart; valueEnd-- {
					if tokens[valueEnd].Kind != Whitespace {
						break
					}
				}
				return keyEnd, equalsIdx, valueStart, valueEnd, commentStart
			}
		case Newline:
			if depth == 0 {
				// Value ends at last non-whitespace before newline.
				for valueEnd = i - 1; valueEnd >= valueStart; valueEnd-- {
					if tokens[valueEnd].Kind != Whitespace {
						break
					}
				}
				return keyEnd, equalsIdx, valueStart, valueEnd, -1
			}
		default:
			// Other token kinds are part of the value — continue scanning.
		}
	}

	// End of tokens without newline.
	for valueEnd = len(tokens) - 1; valueEnd >= valueStart; valueEnd-- {
		if tokens[valueEnd].Kind != Whitespace && tokens[valueEnd].Kind != Newline {
			break
		}
	}
	return keyEnd, equalsIdx, valueStart, valueEnd, commentStart
}

// sortEntry pairs an entry group with its preceding comments for sorting.
type sortEntry struct {
	comments []Group // comments preceding this entry
	entry    Group   // the key=value entry
}

// sortGroups sorts consecutive entry groups (not separated by blank lines
// or table headers) alphabetically by key. Comments preceding an entry
// travel with that entry.
func (*Printer) sortGroups(groups []Group) []Group {
	result := make([]Group, 0, len(groups))
	var entryRun []sortEntry
	var commentRun []Group

	flushEntries := func() {
		if len(entryRun) > 0 {
			slices.SortStableFunc(entryRun, func(a, b sortEntry) int {
				aKey := extractKey(a.entry)
				bKey := extractKey(b.entry)
				return strings.Compare(aKey, bKey)
			})
			for _, se := range entryRun {
				result = append(result, se.comments...)
				result = append(result, se.entry)
			}
			entryRun = nil
		}
	}

	for _, group := range groups {
		switch group.Kind {
		case GroupEntry:
			// Attach pending comments to this entry.
			entryRun = append(entryRun, sortEntry{
				comments: commentRun,
				entry:    group,
			})
			commentRun = nil

		case GroupComment:
			// Comments might be attached to the next entry.
			commentRun = append(commentRun, group)

		case GroupBlank, GroupTable, GroupArrayTable:
			// Separators break the sort group.
			if len(commentRun) > 0 {
				// Standalone comments (not attached to entry) — flush without sort.
				result = append(result, commentRun...)
				commentRun = nil
			}
			flushEntries()
			result = append(result, group)
		default:
			// Unknown group kind — treat as separator.
			flushEntries()
			result = append(result, group)
		}
	}

	// Flush remaining.
	flushEntries()
	if len(commentRun) > 0 {
		// Trailing comments with no entry — emit as-is.
		result = append(result, commentRun...)
	}

	return result
}

// extractKey returns the key string for sorting from an entry group.
// For dotted keys like a.b.c, returns "a.b.c".
// For comment groups, returns "" (they stay with adjacent entries).
func extractKey(group Group) string {
	if group.Kind == GroupComment {
		return ""
	}
	var b strings.Builder
	pastLeadingWS := false
	for _, tok := range group.Tokens {
		if tok.Kind == Whitespace && !pastLeadingWS {
			continue // skip leading whitespace (indentation)
		}
		if tok.Kind == Equals || (tok.Kind == Whitespace && pastLeadingWS) {
			break
		}
		switch tok.Kind {
		case BareKey, BasicString, LiteralString:
			pastLeadingWS = true
			_, _ = b.Write(tok.Raw)
		case Dot:
			pastLeadingWS = true
			_ = b.WriteByte('.')
		default:
			// Other token kinds not part of the key — skip.
		}
	}
	return b.String()
}

// writeNewline writes the configured line ending.
func (p *Printer) writeNewline() {
	if p.opts.LineEnding == formatter.LineEndingCRLF {
		p.buf.WriteString("\r\n")
	} else {
		p.buf.WriteByte('\n')
	}
}

// Pre-computed double newline sequences to avoid allocation in ensureBlankLine.
var (
	doubleLF   = []byte("\n\n")
	doubleCRLF = []byte("\r\n\r\n")
)

// ensureBlankLine ensures there's at least one blank line at the current
// position in the output buffer.
func (p *Printer) ensureBlankLine() {
	out := p.buf.Bytes()
	if len(out) == 0 {
		return
	}

	var nl []byte
	var doubleNL []byte
	if p.opts.LineEnding == formatter.LineEndingCRLF {
		nl = []byte("\r\n")
		doubleNL = doubleCRLF
	} else {
		nl = []byte("\n")
		doubleNL = doubleLF
	}

	if bytes.HasSuffix(out, doubleNL) {
		return
	}
	if bytes.HasSuffix(out, nl) {
		p.writeNewline()
		return
	}
	p.writeNewline()
	p.writeNewline()
}
