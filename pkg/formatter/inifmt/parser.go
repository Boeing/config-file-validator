package inifmt

// Entry represents a key-value pair in an INI file with its surrounding context.
type Entry struct {
	LeadingComments []Token // comment lines preceding this entry
	Key             Token   // the key token
	Sep             Token   // the separator token (= or : with whitespace)
	Value           Token   // the value token (may be empty/zero)
	Newline         Token   // trailing newline
	BlankBefore     bool    // blank line preceded this entry
}

// Section represents a named section in an INI file.
type Section struct {
	LeadingComments []Token // comment lines preceding the section header
	Header          Token   // [section-name] token (zero for default section)
	HeaderNewline   Token   // newline after section header
	Entries         []Entry
	BlankBefore     bool // blank line preceded this section
}

// File represents the complete parsed structure of an INI file.
type File struct {
	Sections []Section // first section may have zero Header (default section)
	Trailing []Token   // trailing comments/blanks after last entry
}

// parse builds a File structure from a flat token stream.
// It organizes tokens into sections and entries, associating comments
// with the item that follows them.
//
// Blank lines (standalone TokNewline) are tracked as separators between
// groups but not emitted as tokens — the printer handles spacing.
func parse(tokens []Token) *File {
	f := &File{}
	var pendingComments []Token
	blankBefore := false

	// Start with a default section (no header).
	currentSection := &Section{}
	i := 0

	for i < len(tokens) {
		tok := tokens[i]

		switch tok.Kind {
		case TokNewline:
			// Standalone newline = blank line separator.
			blankBefore = true
			i++

		case TokComment:
			// Collect comment (without its trailing newline — the printer adds that).
			pendingComments = append(pendingComments, tok)
			i++
			// Consume following newline (part of the comment's line, not a blank line).
			if i < len(tokens) && tokens[i].Kind == TokNewline {
				i++
			}

		case TokSection:
			// New section header — save current section and start new one.
			f.Sections = append(f.Sections, *currentSection)
			currentSection = &Section{
				LeadingComments: pendingComments,
				Header:          tok,
				BlankBefore:     blankBefore,
			}
			pendingComments = nil
			blankBefore = false
			i++
			// Consume following newline if present.
			if i < len(tokens) && tokens[i].Kind == TokNewline {
				currentSection.HeaderNewline = tokens[i]
				i++
			}

		case TokKey:
			// Key-value entry.
			e := Entry{
				LeadingComments: pendingComments,
				Key:             tok,
				BlankBefore:     blankBefore,
			}
			pendingComments = nil
			blankBefore = false
			i++

			// Separator.
			if i < len(tokens) && tokens[i].Kind == TokSeparator {
				e.Sep = tokens[i]
				i++
			}

			// Value.
			if i < len(tokens) && tokens[i].Kind == TokValue {
				e.Value = tokens[i]
				i++
			}

			// Trailing newline.
			if i < len(tokens) && tokens[i].Kind == TokNewline {
				e.Newline = tokens[i]
				i++
			}

			currentSection.Entries = append(currentSection.Entries, e)

		default:
			// Skip whitespace and unexpected tokens — formatting re-applies indent.
			i++
		}
	}

	// Close the last section.
	f.Sections = append(f.Sections, *currentSection)

	// Any pending comments at end become trailing.
	f.Trailing = pendingComments

	return f
}
