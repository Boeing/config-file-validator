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
	// inInlineTable is true while printing the contents of an inline table,
	// where newlines are not allowed by the TOML spec.
	inInlineTable bool
	// inlineTableLineLen is the estimated single-line length of the current
	// inline table (including key prefix). Used to decide whether nested arrays
	// should be expanded to reduce line width.
	inlineTableLineLen int
	// inExpandedArray is true when printing elements of a multiline array.
	// Nested arrays should also be expanded (taplo behavior).
	inExpandedArray bool
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
	alignedCommentColumns := findAlignedCommentColumns(groups, p.opts.Indent)

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
			// Taplo does NOT add blank lines before tables — it preserves
			// source blank lines (which are already handled as GroupBlank above).
			// Per taplo spec: allowed_blank_lines controls preservation, not insertion.
			p.printTableHeader(group)

		case GroupEntry:
			started = true
			entryDepth := 0
			if inTable && p.opts.Indent != "" {
				p.buf.WriteString(p.opts.Indent)
				entryDepth = 1
			}
			p.printEntry(group, entryDepth, alignedCommentColumns[i])
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
func (p *Printer) printEntry(group Group, depth, commentColumn int) {
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
	// Include trailing comment width so arrays expand when the full line > 80.
	keyLen := 0
	for i := 0; i <= keyEnd; i++ {
		if tokens[i].Kind != Whitespace {
			keyLen += len(tokens[i].Raw)
		}
	}
	prefixLen := len(p.opts.Indent) + keyLen + 3 // 3 for " = "
	if commentStart >= 0 {
		// Add minimum spacing (1) + comment length to the width budget.
		for i := commentStart; i < len(tokens); i++ {
			if tokens[i].Kind == Comment {
				prefixLen += 1 + len(tokens[i].Raw) // space + "# comment text"
			}
		}
	}
	p.printValue(tokens[valueStart:valueEnd+1], depth, prefixLen)

	// Emit trailing comment if present.
	if commentStart >= 0 {
		p.writeCommentSpacing(commentColumn)
		for i := commentStart; i < len(tokens); i++ {
			tok := tokens[i]
			if tok.Kind == Comment {
				p.buf.Write(tok.Raw)
			}
		}
	}

	p.writeNewline()
}

// findAlignedCommentColumns computes the target column for aligned inline
// comments in runs of consecutive entries. For groups of 2+ entries that all
// have inline comments, comments are aligned to the column after the widest
// key=value portion. This matches taplo's behavior.
//
// The formatted width of an entry is: indent + keyLen + " = " + valueLen.
// For entries with inline comments, values are always simple scalars (you
// can't have a multiline array and an inline comment on the same TOML line).
func findAlignedCommentColumns(groups []Group, indent string) []int {
	columns := make([]int, len(groups))

	// Determine which entries are inside a table (and get indent in output).
	insideTable := make([]bool, len(groups))
	inTable := false
	for i, g := range groups {
		if g.Kind == GroupTable || g.Kind == GroupArrayTable {
			inTable = true
		}
		if g.Kind == GroupEntry {
			insideTable[i] = inTable
		}
	}

	for start := 0; start < len(groups); {
		_, ok := inlineCommentColumn(groups[start])
		if !ok {
			start++
			continue
		}
		// Find run of consecutive entries with inline comments.
		end := start + 1
		for end < len(groups) {
			if _, nextOK := inlineCommentColumn(groups[end]); !nextOK {
				break
			}
			end++
		}
		if end-start >= 2 {
			// Compute max formatted width (indent + key + " = " + value).
			maxWidth := 0
			for i := start; i < end; i++ {
				w := entryKeyValueWidth(groups[i])
				if insideTable[i] {
					w += len(indent)
				}
				if w > maxWidth {
					maxWidth = w
				}
			}
			// Target = max width + 1 space padding before #.
			for i := start; i < end; i++ {
				columns[i] = maxWidth + 1
			}
		}
		start = end
	}
	return columns
}

// entryKeyValueWidth computes the formatted width of an entry's key = value
// portion (excluding comment). This is: keyLen + " = " + valueLen.
// Does NOT include indent (that's added by the printer's Print loop).
func entryKeyValueWidth(group Group) int {
	if group.Kind != GroupEntry {
		return 0
	}
	tokens := group.Tokens
	keyEnd, equalsIdx, valueStart, valueEnd, _ := splitEntry(tokens)
	if equalsIdx < 0 || valueStart < 0 {
		return 0
	}
	// Key width (raw non-whitespace bytes).
	width := 0
	for i := 0; i <= keyEnd; i++ {
		if tokens[i].Kind != Whitespace {
			width += len(tokens[i].Raw)
		}
	}
	width += 3 // " = "
	// Value width (all raw bytes between valueStart and valueEnd).
	for i := valueStart; i <= valueEnd; i++ {
		width += len(tokens[i].Raw)
	}
	return width
}

func inlineCommentColumn(group Group) (int, bool) {
	if group.Kind != GroupEntry {
		return 0, false
	}
	column := 0
	for _, tok := range group.Tokens {
		if tok.Kind == Comment {
			return column, true
		}
		if tok.Kind == Newline {
			return 0, false
		}
		column += len(tok.Raw)
	}
	return 0, false
}

func (p *Printer) writeCommentSpacing(targetColumn int) {
	spaces := 1
	if targetColumn > 0 {
		lineStart := bytes.LastIndexByte(p.buf.Bytes(), '\n') + 1
		currentColumn := p.buf.Len() - lineStart
		if targetColumn > currentColumn {
			spaces = targetColumn - currentColumn
		}
	}
	p.buf.WriteString(strings.Repeat(" ", spaces))
}

// printValue writes value tokens. For simple values, emit verbatim.
// For arrays and inline tables, normalize internal spacing.
func (p *Printer) printValue(tokens []Token, depth int, prefixLen int) {
	if len(tokens) == 0 {
		return
	}

	// If value contains comments AND it's not an array, emit verbatim.
	// Arrays handle comments correctly in printArrayMultiline.
	// Inline tables and scalars with comments are too complex to normalize.
	if tokens[0].Kind != BracketOpen {
		for _, tok := range tokens {
			if tok.Kind == Comment {
				for _, t := range tokens {
					p.buf.Write(t.Raw)
				}
				return
			}
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
	// - Inside inline table that exceeds column width → expand arrays to reduce width
	// - Fits → stay inline
	effectivePrefix := prefixLen
	if p.inInlineTable && p.inlineTableLineLen > p.opts.ColumnWidth {
		// The inline table exceeds column width. Use the full line length as
		// the effective prefix so any non-trivial array will be expanded.
		effectivePrefix = p.inlineTableLineLen
	}
	multiline := hasComments || p.inExpandedArray || (effectivePrefix+singleLineLen) > p.opts.ColumnWidth

	if multiline {
		// When expanding an array inside an inline table, use depth 0 so
		// elements indent relative to line start (matching taplo behavior).
		arrayDepth := depth
		if p.inInlineTable {
			arrayDepth = 0
		}
		p.printArrayMultiline(elements, arrayDepth)
	} else {
		p.printArrayInline(elements, depth)
	}
}

// printArrayInline writes an array on a single line: [elem, elem, elem]
func (p *Printer) printArrayInline(elements [][]Token, depth int) {
	p.buf.WriteByte('[')
	for i, elem := range elements {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.printValue(trimValueTokens(elem), depth, 0)
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
	wasExpanded := p.inExpandedArray
	p.inExpandedArray = true

	// Pre-compute max value width for trailing comment alignment.
	// Only align if 2+ elements have trailing comments.
	maxValueWidth := 0
	trailingCommentCount := 0
	for _, elem := range elements {
		seenVal := false
		hasTrailing := false
		valWidth := len(elemIndent)
		for _, tok := range elem {
			switch tok.Kind {
			case Newline:
				seenVal = false
			case Comment:
				if seenVal {
					hasTrailing = true
				}
			case Whitespace:
				// don't count
			default:
				seenVal = true
				valWidth += len(tok.Raw)
			}
		}
		if hasTrailing {
			trailingCommentCount++
			valWidth++ // for the comma
			if valWidth > maxValueWidth {
				maxValueWidth = valWidth
			}
		}
	}
	alignComments := trailingCommentCount >= 2

	for i, elem := range elements {
		p.writeNewline()
		// Separate comments into leading (own line) and trailing (after value).
		// A comment is trailing if it appears AFTER value tokens with no
		// intervening Newline (it was on the same source line as the value).
		var leadingComments []Token
		var trailingComment *Token
		var valueTokens []Token
		seenValue := false
		for _, tok := range elem {
			switch tok.Kind {
			case Newline:
				seenValue = false
			case Comment:
				if seenValue {
					t := tok
					trailingComment = &t
				} else {
					leadingComments = append(leadingComments, tok)
				}
			default:
				if tok.Kind != Whitespace {
					seenValue = true
				}
				valueTokens = append(valueTokens, tok)
			}
		}
		valueTokens = trimValueTokens(valueTokens)
		// Emit leading comments on their own lines.
		for _, c := range leadingComments {
			p.buf.WriteString(elemIndent)
			p.buf.Write(c.Raw)
			p.writeNewline()
		}
		// Emit value.
		if len(valueTokens) > 0 {
			p.buf.WriteString(elemIndent)
			p.printValue(valueTokens, depth+1, p.column())
			if p.opts.TrailingComma || i < len(elements)-1 {
				p.buf.WriteByte(',')
			}
			// Emit trailing comment on the same line.
			if trailingComment != nil {
				if alignComments {
					// Pad to align with other trailing comments in this array.
					lineStart := bytes.LastIndexByte(p.buf.Bytes(), '\n') + 1
					currentCol := p.buf.Len() - lineStart
					spaces := 1
					if maxValueWidth+1 > currentCol {
						spaces = maxValueWidth + 1 - currentCol
					}
					p.buf.WriteString(strings.Repeat(" ", spaces))
				} else {
					p.buf.WriteString("  ")
				}
				p.buf.Write(trailingComment.Raw)
			}
		}
	}
	p.inExpandedArray = wasExpanded
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
	wasInline := p.inInlineTable
	prevLineLen := p.inlineTableLineLen
	p.inInlineTable = true
	// Estimate the total line width of this inline table (from current column
	// position). This allows nested arrays to expand when the line is too long.
	p.inlineTableLineLen = p.column() + estimateInlineTableWidth(content)
	defer func() {
		p.inInlineTable = wasInline
		p.inlineTableLineLen = prevLineLen
	}()

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
	p.printValue(valueTokens, depth+1, p.column())
}

// trimValueTokens removes leading and trailing whitespace/newline tokens.
func trimValueTokens(tokens []Token) []Token {
	return trimTrailingWhitespace(trimLeadingWhitespace(tokens))
}

// column returns the number of bytes written since the last newline, i.e. the
// column the next byte will land on.
func (p *Printer) column() int {
	out := p.buf.Bytes()
	if i := bytes.LastIndexByte(out, '\n'); i >= 0 {
		return len(out) - i - 1
	}
	return len(out)
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

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.Kind {
		case BracketOpen, BraceOpen:
			depth++
			current = append(current, tok)
		case BracketClose, BraceClose:
			depth--
			current = append(current, tok)
		case Comma:
			if depth == 0 {
				// Include same-line trailing content (whitespace + comment
				// before next newline) with this element. This keeps trailing
				// comments attached to their value.
				for i+1 < len(tokens) {
					next := tokens[i+1]
					if next.Kind == Newline || next.Kind == BracketClose || next.Kind == BraceClose {
						break
					}
					if next.Kind != Whitespace && next.Kind != Comment {
						break
					}
					current = append(current, next)
					i++
				}
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

// estimateInlineTableWidth estimates the single-line character width of an
// inline table's content (excluding the key prefix). Used to determine whether
// nested arrays should be expanded.
func estimateInlineTableWidth(tokens []Token) int {
	// Estimate: "{ " + key1 + " = " + val1 + ", " + key2 + " = " + val2 + " }"
	// Count raw token bytes plus the normalized spacing that printInlineTable adds:
	// - Each "=" becomes " = " (+2 chars)
	// - Each "," becomes ", " (+1 char)
	length := 4 // "{ " + " }"
	for _, tok := range tokens {
		if tok.Kind == Whitespace || tok.Kind == Newline || tok.Kind == Comment {
			continue
		}
		length += len(tok.Raw)
		switch tok.Kind {
		case Equals:
			length += 2 // " = " instead of "="
		case Comma:
			length++ // ", " instead of ","
		default:
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
// extractTableKey returns the dotted key path from a table header group.
// For [build.buildStats], returns "build.buildStats".
// For [[build.cachebusters]], returns "build.cachebusters".

// tableParent returns the parent key of a dotted table key.
// "build.buildStats" → "build", "target.x86_64-msvc" → "target", "root" → ""

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
