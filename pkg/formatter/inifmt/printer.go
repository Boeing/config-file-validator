package inifmt

import (
	"bytes"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// printFormatted renders the parsed File structure into canonical INI output.
func printFormatted(f *File, opts formatter.Options) []byte {
	indent := buildIndent(opts)
	var buf bytes.Buffer

	for secIdx, sec := range f.Sections {
		// Skip the default section if it has no content (no header, no entries).
		if secIdx == 0 && len(sec.Header.Raw) == 0 && len(sec.Entries) == 0 && len(sec.LeadingComments) == 0 {
			continue
		}

		// Blank line before sections (except the very first output line).
		if len(sec.Header.Raw) > 0 && buf.Len() > 0 {
			buf.WriteByte('\n')
		}

		// Emit section leading comments.
		for _, c := range sec.LeadingComments {
			buf.Write(c.Raw)
			buf.WriteByte('\n')
		}

		// Emit section header.
		if len(sec.Header.Raw) > 0 {
			buf.Write(sec.Header.Raw)
			buf.WriteByte('\n')
		}

		// Sort entries if requested.
		entries := sec.Entries
		if opts.SortKeys {
			entries = sortEntries(entries)
		}

		// Emit entries.
		for _, e := range entries {
			// Emit entry leading comments with indent.
			for _, c := range e.LeadingComments {
				if indent != "" && len(sec.Header.Raw) > 0 {
					buf.WriteString(indent)
				}
				buf.Write(c.Raw)
				buf.WriteByte('\n')
			}

			// Emit key with indent.
			if indent != "" && len(sec.Header.Raw) > 0 {
				buf.WriteString(indent)
			}
			buf.Write(e.Key.Raw)

			// Emit separator and value.
			// Preserve original separator when value starts with a quote.
			// ini.v1's PreserveSurroundedQuote option interprets "key=\"val\""
			// differently from "key = \"val\"" — introducing whitespace between
			// = and a quote can change semantics.
			if len(e.Sep.Raw) > 0 {
				if valueStartsWithQuote(e.Value.Raw) {
					buf.Write(e.Sep.Raw)
				} else {
					sep := findSepChar(e.Sep.Raw)
					buf.WriteString(" " + string(sep) + " ")
				}
			}

			// Emit value verbatim.
			if len(e.Value.Raw) > 0 {
				buf.Write(e.Value.Raw)
			}

			buf.WriteByte('\n')
		}
	}

	// Emit trailing comments.
	for _, c := range f.Trailing {
		buf.Write(c.Raw)
		buf.WriteByte('\n')
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

// valueStartsWithQuote returns true if the value token starts with " or '.
func valueStartsWithQuote(raw []byte) bool {
	return len(raw) > 0 && (raw[0] == '"' || raw[0] == '\'')
}

// findSepChar extracts the actual separator character (= or :) from a
// separator token that may include surrounding whitespace.
func findSepChar(raw []byte) byte {
	for _, b := range raw {
		if b == '=' || b == ':' {
			return b
		}
	}
	return '='
}

// sortEntries returns a sorted copy of entries, sorted by decoded key.
// Comments attached to entries travel with them.
func sortEntries(entries []Entry) []Entry {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	slices.SortStableFunc(sorted, func(a, b Entry) int {
		aKey := decodeKey(a.Key.Raw)
		bKey := decodeKey(b.Key.Raw)
		return strings.Compare(aKey, bKey)
	})
	return sorted
}

// decodeKey returns the key text for sorting purposes.
// INI keys do not have escape sequences — backslashes are literal.
func decodeKey(raw []byte) string {
	return string(raw)
}

// buildIndent constructs the indent string from options.
func buildIndent(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	if opts.IndentWidth <= 0 {
		return ""
	}
	return strings.Repeat(" ", opts.IndentWidth)
}
