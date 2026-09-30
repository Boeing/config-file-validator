package tomlfmt

import (
	"strings"
	"testing"
)

func TestTokenizeSimple(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect []TokenKind
	}{
		{"empty", "", nil},
		{"newline", "\n", []TokenKind{Newline}},
		{"crlf", "\r\n", []TokenKind{Newline}},
		{"whitespace", "  \t", []TokenKind{Whitespace}},
		{"comment", "# hello", []TokenKind{Comment}},
		{"comment with newline", "# hello\n", []TokenKind{Comment, Newline}},
		{"punctuation", "[]{}.,=", []TokenKind{BracketOpen, BracketClose, BraceOpen, BraceClose, Dot, Comma, Equals}},
		{"bare key", "my_key-1", []TokenKind{BareKey}},
		{"bool true", "true", []TokenKind{Bool}},
		{"bool false", "false", []TokenKind{Bool}},
		{"integer", "42", []TokenKind{Integer}},
		{"negative int", "-17", []TokenKind{Integer}},
		{"hex", "0xFF", []TokenKind{Integer}},
		{"oct", "0o77", []TokenKind{Integer}},
		{"bin", "0b1010", []TokenKind{Integer}},
		{"float", "3.14", []TokenKind{Float}},
		{"float exp", "1e10", []TokenKind{Float}},
		{"nan", "nan", []TokenKind{Float}},
		{"inf", "inf", []TokenKind{Float}},
		{"+inf", "+inf", []TokenKind{Float}},
		{"-inf", "-inf", []TokenKind{Float}},
		{"basic string", `"hello"`, []TokenKind{BasicString}},
		{"empty basic string", `""`, []TokenKind{BasicString}},
		{"string with escape", `"he\"llo"`, []TokenKind{BasicString}},
		{"literal string", `'hello'`, []TokenKind{LiteralString}},
		{"multiline basic", `"""hello"""`, []TokenKind{MultiLineBasicString}},
		{"multiline literal", `'''hello'''`, []TokenKind{MultiLineLiteralString}},
		{"datetime", "2024-01-15T10:30:00Z", []TokenKind{DateTime}},
		{"local date", "2024-01-15", []TokenKind{DateTime}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := NewLexer([]byte(tc.input))
			tokens := l.Tokenize()

			if len(tokens) != len(tc.expect) {
				t.Fatalf("expected %d tokens, got %d: %v", len(tc.expect), len(tokens), tokenKinds(tokens))
			}

			for i, tok := range tokens {
				if tok.Kind != tc.expect[i] {
					t.Errorf("token[%d]: expected %d, got %d (raw=%q)", i, tc.expect[i], tok.Kind, tok.Raw)
				}
			}
		})
	}
}

func TestTokenizeKeyValue(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect []TokenKind
	}{
		{"simple", "key = \"value\"\n", []TokenKind{BareKey, Whitespace, Equals, Whitespace, BasicString, Newline}},
		{"no spaces", "key=\"value\"\n", []TokenKind{BareKey, Equals, BasicString, Newline}},
		{"dotted key", "a.b.c = 1\n", []TokenKind{BareKey, Dot, BareKey, Dot, BareKey, Whitespace, Equals, Whitespace, Integer, Newline}},
		{"quoted key", `"a.b" = 1`, []TokenKind{BasicString, Whitespace, Equals, Whitespace, Integer}},
		{"with comment", "key = 1 # comment\n", []TokenKind{BareKey, Whitespace, Equals, Whitespace, Integer, Whitespace, Comment, Newline}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := NewLexer([]byte(tc.input))
			tokens := l.Tokenize()

			if len(tokens) != len(tc.expect) {
				t.Fatalf("expected %d tokens, got %d: %v\ntokens: %s", len(tc.expect), len(tokens), tokenKinds(tokens), tokenDetail(tokens))
			}

			for i, tok := range tokens {
				if tok.Kind != tc.expect[i] {
					t.Errorf("token[%d]: expected %d, got %d (raw=%q)", i, tc.expect[i], tok.Kind, tok.Raw)
				}
			}
		})
	}
}

func TestTokenizeTable(t *testing.T) {
	input := "[package]\nname = \"myapp\"\nversion = \"1.0\"\n\n[[bin]]\nname = \"cli\"\n"
	l := NewLexer([]byte(input))
	tokens := l.Tokenize()

	// Verify all bytes accounted for
	total := 0
	for _, tok := range tokens {
		total += len(tok.Raw)
	}
	if total != len(input) {
		t.Fatalf("byte coverage: %d tokens cover %d bytes, input is %d bytes", len(tokens), total, len(input))
	}

	// Verify reconstruction
	var reconstructed strings.Builder
	for _, tok := range tokens {
		_, _ = reconstructed.Write(tok.Raw)
	}
	if reconstructed.String() != input {
		t.Fatalf("reconstruction failed:\n  input:  %q\n  output: %q", input, reconstructed.String())
	}
}

func TestTokenizeMultilineStrings(t *testing.T) {
	cases := []struct {
		name  string
		input string
		kind  TokenKind
	}{
		{"basic multiline", "\"\"\"hello\nworld\"\"\"", MultiLineBasicString},
		{"literal multiline", "'''hello\nworld'''", MultiLineLiteralString},
		{"basic with escape", "\"\"\"hello\\\"world\"\"\"", MultiLineBasicString},
		{"basic with 1 extra quote", "\"\"\"\"hello\"\"\"\"", MultiLineBasicString},      // content: "hello"
		{"basic with 2 extra quotes", "\"\"\"\"\"hello\"\"\"\"\"", MultiLineBasicString}, // content: ""hello""
		{"literal with 1 extra quote", "''''hello''''", MultiLineLiteralString},
		{"literal with 2 extra quotes", "'''''hello'''''", MultiLineLiteralString},
		{"empty multiline basic", "\"\"\"\"\"\"", MultiLineBasicString},
		{"empty multiline literal", "''''''", MultiLineLiteralString},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := NewLexer([]byte(tc.input))
			tokens := l.Tokenize()

			if len(tokens) == 0 {
				t.Fatal("no tokens produced")
			}
			if tokens[0].Kind != tc.kind {
				t.Errorf("expected kind %d, got %d", tc.kind, tokens[0].Kind)
			}
			if string(tokens[0].Raw) != tc.input {
				t.Errorf("expected entire input as one token, got %q (input was %q)", tokens[0].Raw, tc.input)
			}
		})
	}
}

func TestTokenizeBytesCovered(t *testing.T) {
	// Every valid TOML input should have all bytes accounted for.
	inputs := []string{
		"",
		"# just a comment\n",
		"[table]\nkey = \"value\"\n",
		"arr = [1, 2, 3]\n",
		"inline = {a = 1, b = 2}\n",
		"ml = \"\"\"\nhello\nworld\n\"\"\"\n",
		"lit = '''\nhello\n'''\n",
		"date = 2024-01-15T10:30:00Z\n",
		"hex = 0xDEAD_BEEF\n",
		"[a.b.c]\nkey = true\n",
	}

	for _, input := range inputs {
		l := NewLexer([]byte(input))
		tokens := l.Tokenize()

		total := 0
		for _, tok := range tokens {
			total += len(tok.Raw)
		}
		if total != len(input) {
			t.Errorf("input %q: %d bytes in tokens vs %d bytes in input", input, total, len(input))
		}

		// Verify reconstruction
		var b strings.Builder
		for _, tok := range tokens {
			_, _ = b.Write(tok.Raw)
		}
		if b.String() != input {
			t.Errorf("input %q: reconstruction mismatch: %q", input, b.String())
		}
	}
}

// FuzzTokenizer verifies:
// - No panics on any input
// - Every byte is accounted for (total token bytes == input bytes)
// - Reconstruction from tokens equals original input
func FuzzTokenizer(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("[table]\nkey = \"value\"\n"))
	f.Add([]byte("# comment\n"))
	f.Add([]byte("\"\"\"multiline\n\"\"\""))
	f.Add([]byte("'''literal\n'''"))
	f.Add([]byte("key = [1, 2, 3]\n"))
	f.Add([]byte("a.b.c = {x = 1}\n"))
	f.Add([]byte("d = 2024-01-15T10:30:00Z\n"))
	f.Add([]byte("\"\"\"\"\"hello\"\"\"\"\""))
	f.Add([]byte("'''''hello'''''"))
	f.Add([]byte{0xFF, 0xFE, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		l := NewLexer(data)
		tokens := l.Tokenize()

		// All bytes must be accounted for.
		total := 0
		for _, tok := range tokens {
			total += len(tok.Raw)
		}
		if total != len(data) {
			t.Fatalf("byte coverage mismatch: tokens=%d, input=%d", total, len(data))
		}

		// Reconstruction must equal original.
		var b []byte
		for _, tok := range tokens {
			b = append(b, tok.Raw...)
		}
		if string(b) != string(data) {
			t.Fatal("reconstruction mismatch")
		}
	})
}

// helpers

func tokenKinds(tokens []Token) []TokenKind {
	kinds := make([]TokenKind, len(tokens))
	for i, t := range tokens {
		kinds[i] = t.Kind
	}
	return kinds
}

func tokenDetail(tokens []Token) string {
	var b strings.Builder
	for i, t := range tokens {
		if i > 0 {
			_, _ = b.WriteString(", ")
		}
		_, _ = b.WriteString(tokenKindName(t.Kind))
		_, _ = b.WriteString("(")
		_, _ = b.Write(t.Raw)
		_, _ = b.WriteString(")")
	}
	return b.String()
}

func tokenKindName(k TokenKind) string {
	switch k {
	case Whitespace:
		return "WS"
	case Newline:
		return "NL"
	case Comment:
		return "CMT"
	case BareKey:
		return "KEY"
	case BasicString:
		return "STR"
	case MultiLineBasicString:
		return "MLSTR"
	case LiteralString:
		return "LSTR"
	case MultiLineLiteralString:
		return "MLLSTR"
	case Integer:
		return "INT"
	case Float:
		return "FLT"
	case Bool:
		return "BOOL"
	case DateTime:
		return "DT"
	case Dot:
		return "DOT"
	case Comma:
		return "COMMA"
	case Equals:
		return "EQ"
	case BracketOpen:
		return "["
	case BracketClose:
		return "]"
	case BraceOpen:
		return "{"
	case BraceClose:
		return "}"
	default:
		return "???"
	}
}

func TestTokenizerStress(t *testing.T) {
	// Inputs that might cause infinite loops
	stress := []string{
		string([]byte{0xFF, 0xFE, 0x00}),
		"\"",
		"'",
		"\"\"\"",
		"'''",
		"\"\"\"\"",
		"''''",
		strings.Repeat("\"", 100),
		strings.Repeat("'", 100),
		strings.Repeat("\\", 100),
		"\"" + strings.Repeat("\\\"", 1000) + "\"",
	}

	for i, s := range stress {
		l := NewLexer([]byte(s))
		tokens := l.Tokenize()

		total := 0
		for _, tok := range tokens {
			total += len(tok.Raw)
		}
		if total != len(s) {
			t.Errorf("case %d: byte mismatch: got %d, want %d", i, total, len(s))
		}
	}
}

func TestSortKeysCommentAttachment(t *testing.T) {
	input := "# comment for z\nz = 3\n# comment for a\na = 1\n" //nolint:dupword // test data, not prose
	opts := DefaultOptions()
	opts.SortKeys = true

	f := Formatter{}
	out, err := f.Format([]byte(input), opts)
	if err != nil {
		t.Fatal(err)
	}

	result := string(out)
	t.Logf("Output: %q", result)

	// "a" should come before "z"
	aIdx := strings.Index(result, "a = 1")
	zIdx := strings.Index(result, "z = 3")
	if aIdx > zIdx {
		t.Error("a should come before z")
	}

	// Comment for a should come before a
	aCommentIdx := strings.Index(result, "# comment for a")
	if aCommentIdx > aIdx {
		t.Errorf("comment for a (pos %d) should come before a = 1 (pos %d)", aCommentIdx, aIdx)
	}

	// Comment for z should come before z
	zCommentIdx := strings.Index(result, "# comment for z")
	if zCommentIdx > zIdx {
		t.Errorf("comment for z (pos %d) should come before z = 3 (pos %d)", zCommentIdx, zIdx)
	}
}

func TestTableHeaderInlineComment(t *testing.T) {
	input := "[section] # section comment\nkey = 1\n"
	f := Formatter{}
	out, err := f.Format([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	result := string(out)
	t.Logf("Output: %q", result)

	if !strings.Contains(result, "[section] # section comment") {
		t.Errorf("expected space before comment in table header, got: %s", result)
	}
}

func TestMultiLineCommentIndent(t *testing.T) {
	input := "[table]\n# line 1\n# line 2\nkey = 1\n"
	opts := DefaultOptions()
	opts.IndentWidth = 4

	f := Formatter{}
	out, err := f.Format([]byte(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	result := string(out)
	t.Logf("Output:\n%s", result)

	if !strings.Contains(result, "    # line 1") {
		t.Error("expected 4-space indent on # line 1")
	}
	if !strings.Contains(result, "    # line 2") {
		t.Error("expected 4-space indent on # line 2")
	}
	if !strings.Contains(result, "    key = 1") {
		t.Error("expected 4-space indent on key = 1")
	}
}

func TestLongArrayExpand(t *testing.T) {
	// This line is >80 chars with the key prefix
	input := "[t]\narray_is_a_bit_too_long = [\"this_line_is_80_characters_long_xxxxx\", \"filler_data\"]\n"
	f := Formatter{}
	opts := DefaultOptions()
	out, err := f.Format([]byte(input), opts)
	if err != nil {
		t.Fatal(err)
	}
	result := string(out)
	t.Logf("Output: %q", result)

	// Should be multiline because total line exceeds 80 columns
	if !strings.Contains(result, ",\n") {
		t.Error("expected multiline array (exceeds 80 columns)")
	}
}
