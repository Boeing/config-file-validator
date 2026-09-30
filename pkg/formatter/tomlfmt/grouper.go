package tomlfmt

// GroupKind identifies the type of a logical group in a TOML document.
type GroupKind int

const (
	// GroupBlank represents one or more blank lines.
	GroupBlank GroupKind = iota
	// GroupComment represents one or more standalone comment lines.
	GroupComment
	// GroupTable represents a table header [key] or [key.subkey].
	GroupTable
	// GroupArrayTable represents an array table header [[key]].
	GroupArrayTable
	// GroupEntry represents a key = value pair (may span multiple lines).
	GroupEntry
)

// Group is a logical unit in a TOML document.
type Group struct {
	Kind   GroupKind
	Tokens []Token
}

// Grouper classifies a flat token stream into logical groups.
// It tracks bracket/brace depth to correctly identify multiline values
// as part of a single entry group.
type Grouper struct {
	tokens []Token
	pos    int
}

// NewGrouper creates a Grouper for the given token stream.
func NewGrouper(tokens []Token) *Grouper {
	return &Grouper{tokens: tokens}
}

// Group processes all tokens and returns the logical groups.
func (g *Grouper) Group() []Group {
	var groups []Group
	for g.pos < len(g.tokens) {
		group := g.nextGroup()
		if group != nil {
			groups = append(groups, *group)
		}
	}
	return groups
}

// nextGroup consumes tokens for one logical group.
func (g *Grouper) nextGroup() *Group {
	if g.pos >= len(g.tokens) {
		return nil
	}

	tok := g.tokens[g.pos]

	switch tok.Kind {
	case Newline:
		return g.consumeBlank()
	case Whitespace:
		// Whitespace at line start — peek ahead to see what follows.
		return g.consumeAfterWhitespace()
	case Comment:
		return g.consumeComment()
	case BracketOpen:
		return g.consumeTableHeader()
	default:
		return g.consumeEntry()
	}
}

// consumeBlank consumes all consecutive blank content (newlines and
// whitespace-only lines) into a single blank group.
func (g *Grouper) consumeBlank() *Group {
	start := g.pos
	for g.pos < len(g.tokens) {
		tok := g.tokens[g.pos]
		if tok.Kind != Newline && tok.Kind != Whitespace {
			break
		}
		g.pos++
	}
	// Count how many actual newlines are in this blank region.
	// This tells us how many blank lines it represents.
	return &Group{Kind: GroupBlank, Tokens: g.tokens[start:g.pos]}
}

// consumeAfterWhitespace handles whitespace at the start of a line.
// Looks ahead to determine if this is a blank line, indented comment,
// or indented entry.
func (g *Grouper) consumeAfterWhitespace() *Group {
	// Peek past whitespace.
	peek := g.pos + 1
	for peek < len(g.tokens) && g.tokens[peek].Kind == Whitespace {
		peek++
	}

	if peek >= len(g.tokens) {
		// Trailing whitespace at end of file — treat as blank.
		return g.consumeBlank()
	}

	switch g.tokens[peek].Kind {
	case Newline:
		// Whitespace-only line → blank. Let consumeBlank handle it
		// so consecutive blank lines merge into one group.
		return g.consumeBlank()
	case Comment:
		return g.consumeComment()
	case BracketOpen:
		return g.consumeTableHeader()
	default:
		return g.consumeEntry()
	}
}

// consumeComment consumes one or more comment lines.
// A comment line is: optional whitespace + comment token + newline.
func (g *Grouper) consumeComment() *Group {
	start := g.pos
	for g.pos < len(g.tokens) {
		// Skip leading whitespace.
		for g.pos < len(g.tokens) && g.tokens[g.pos].Kind == Whitespace {
			g.pos++
		}
		// Must be a comment.
		if g.pos >= len(g.tokens) || g.tokens[g.pos].Kind != Comment {
			break
		}
		g.pos++ // consume comment
		// Consume trailing newline if present.
		if g.pos < len(g.tokens) && g.tokens[g.pos].Kind == Newline {
			g.pos++
		}
	}
	return &Group{Kind: GroupComment, Tokens: g.tokens[start:g.pos]}
}

// consumeTableHeader consumes a table header [key] or [[key]] including
// its trailing comment and newline.
func (g *Grouper) consumeTableHeader() *Group {
	start := g.pos

	// Consume any leading whitespace.
	for g.pos < len(g.tokens) && g.tokens[g.pos].Kind == Whitespace {
		g.pos++
	}

	// Detect [[ vs [.
	kind := GroupTable
	if g.pos < len(g.tokens) && g.tokens[g.pos].Kind == BracketOpen {
		g.pos++
		if g.pos < len(g.tokens) && g.tokens[g.pos].Kind == BracketOpen {
			kind = GroupArrayTable
			g.pos++
		}
	}

	// Consume until end of line (includes key, closing brackets, comment).
	for g.pos < len(g.tokens) && g.tokens[g.pos].Kind != Newline {
		g.pos++
	}
	// Consume the newline.
	if g.pos < len(g.tokens) && g.tokens[g.pos].Kind == Newline {
		g.pos++
	}

	return &Group{Kind: kind, Tokens: g.tokens[start:g.pos]}
}

// consumeEntry consumes a key = value entry, which may span multiple lines
// if the value is a multiline array, inline table, or multiline string.
func (g *Grouper) consumeEntry() *Group {
	start := g.pos
	depth := 0 // bracket/brace nesting depth
	pastEquals := false

	for g.pos < len(g.tokens) {
		tok := g.tokens[g.pos]

		switch tok.Kind {
		case Equals:
			pastEquals = true
			g.pos++

		case BracketOpen, BraceOpen:
			if pastEquals {
				depth++
			}
			g.pos++

		case BracketClose, BraceClose:
			if pastEquals && depth > 0 {
				depth--
			}
			g.pos++

		case Newline:
			if depth == 0 {
				// End of entry. Consume the newline as part of this group.
				g.pos++
				return &Group{Kind: GroupEntry, Tokens: g.tokens[start:g.pos]}
			}
			// Inside a multiline value — newline is part of the entry.
			g.pos++

		default:
			g.pos++
		}
	}

	// End of input without trailing newline.
	if g.pos > start {
		return &Group{Kind: GroupEntry, Tokens: g.tokens[start:g.pos]}
	}
	return nil
}
