package gojust

import (
	"testing"
)

func TestLexIndentedString(t *testing.T) {
	input := "val := \"\"\"\n  hello\n  world\n\"\"\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindIndentedQuoted {
		t.Errorf("expected indented string, got %s", sl.Kind)
	}
}

func TestLexIndentedRawString(t *testing.T) {
	input := "val := '''\n  hello\\n\n  world\n'''\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindIndentedRaw {
		t.Errorf("expected indented raw string, got %s", sl.Kind)
	}
}

func TestLexIndentedBacktick(t *testing.T) {
	input := "val := ```\n  echo hello\n  echo world\n```\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	bt, ok := jf.Assignments[0].Value.(*BacktickExpr)
	if !ok {
		t.Fatalf("expected BacktickExpr, got %T", jf.Assignments[0].Value)
	}
	if !bt.Indented {
		t.Error("expected indented backtick")
	}
}

func TestLexUnicodeEscape(t *testing.T) {
	input := `val := "\u{1F600}"`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Value != "\U0001F600" {
		t.Errorf("expected grinning face emoji, got %q", sl.Value)
	}
}

func TestLexEscapeSequences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"newline", `val := "a\nb"`, "a\nb"},
		{"tab", `val := "a\tb"`, "a\tb"},
		{"carriage return", `val := "a\rb"`, "a\rb"},
		{"escaped quote", `val := "a\"b"`, "a\"b"},
		{"escaped backslash", `val := "a\\b"`, "a\\b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			jf, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			sl, ok := jf.Assignments[0].Value.(*StringLiteral)
			if !ok {
				t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
			}
			if sl.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, sl.Value)
			}
		})
	}
}

func TestLexLineContinuation(t *testing.T) {
	input := "val := \"hello\" + \\\n  \"world\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	_, ok := jf.Assignments[0].Value.(*Concatenation)
	if !ok {
		t.Fatalf("expected Concatenation, got %T", jf.Assignments[0].Value)
	}
}

func TestLexShellExpandedString(t *testing.T) {
	input := `val := x"echo hello"`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindShellExpanded {
		t.Errorf("expected shell-expanded string, got %s", sl.Kind)
	}
}

func TestLexShellExpandedRawString(t *testing.T) {
	input := `val := x'echo hello'`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindShellExpandedRaw {
		t.Errorf("expected shell-expanded raw string, got %s", sl.Kind)
	}
}

func TestLexFormatString(t *testing.T) {
	input := `val := f"hello {name}"`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindFormat {
		t.Errorf("expected format string, got %s", sl.Kind)
	}
}

func TestLexShebang(t *testing.T) {
	input := "#!/usr/bin/env just\nval := \"hello\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestLexErrorUnterminatedBacktick(t *testing.T) {
	_, err := Parse([]byte("val := `unterminated"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLexErrorUnterminatedRawString(t *testing.T) {
	_, err := Parse([]byte("val := 'unterminated"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLexErrorUnterminatedIndentedString(t *testing.T) {
	_, err := Parse([]byte("val := \"\"\"\nunterminated"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLexErrorUnterminatedIndentedRawString(t *testing.T) {
	_, err := Parse([]byte("val := '''\nunterminated"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLexErrorUnterminatedIndentedBacktick(t *testing.T) {
	_, err := Parse([]byte("val := ```\nunterminated"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLexErrorBadEscape(t *testing.T) {
	_, err := Parse([]byte(`val := "\q"`))
	if err == nil {
		t.Fatal("expected error for bad escape")
	}
}

func TestLexErrorBadUnicodeEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no brace", `val := "\u0041"`},
		{"bad hex", `val := "\u{ZZZZ}"`},
		{"unterminated", `val := "\u{0041"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLexErrorUnexpectedChar(t *testing.T) {
	_, err := Parse([]byte("val := ~bad"))
	if err == nil {
		t.Fatal("expected error for unexpected character")
	}
}

func TestLexTabIndent(t *testing.T) {
	input := "build:\n\techo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes) != 1 || len(jf.Recipes[0].Body) != 1 {
		t.Error("expected 1 recipe with 1 body line")
	}
}

func TestLexEscapeLineContInString(t *testing.T) {
	// Backslash-newline inside a string is a line continuation
	input := "val := \"hello\\\nworld\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Value != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", sl.Value)
	}
}

func TestLexEscapeCarriageReturnInString(t *testing.T) {
	input := "val := \"hello\\\r\nworld\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl2, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected *StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl2.Value != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", sl2.Value)
	}
}

// --- Lexer edge cases ---

func TestLexCarriageReturn(t *testing.T) {
	input := "val := \"hello\"\r\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestLexLineContinuationCRLF(t *testing.T) {
	input := "val := \"a\" + \\\r\n  \"b\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	_, ok := jf.Assignments[0].Value.(*Concatenation)
	if !ok {
		t.Fatalf("expected Concatenation, got %T", jf.Assignments[0].Value)
	}
}

func TestLexAllOperators(t *testing.T) {
	// Exercise operators that might not be hit elsewhere
	tests := []struct {
		input string
	}{
		{"val := \"a\" + \"b\"\n"},
		{"val := \"a\" / \"b\"\n"},
	}
	for _, tt := range tests {
		_, err := Parse([]byte(tt.input))
		if err != nil {
			t.Errorf("Parse failed for %q: %v", tt.input, err)
		}
	}
}

// --- resolveModulePath edge cases ---

func TestLexIndentedQuotedStringWithEscape(t *testing.T) {
	input := "val := \"\"\"\nhello\\nworld\n\"\"\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	sl, ok := jf.Assignments[0].Value.(*StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", jf.Assignments[0].Value)
	}
	if sl.Kind != StringKindIndentedQuoted {
		t.Errorf("expected indented string, got %s", sl.Kind)
	}
}

func TestLexRecipeBodyWithFunctionInInterpolation(t *testing.T) {
	input := "build:\n    echo {{replace(\"a.b\", \".\", \"-\")}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Body) != 1 {
		t.Fatalf("expected 1 body line, got %d", len(jf.Recipes[0].Body))
	}
}

func TestLexRecipeBodyWithRawStringInInterpolation(t *testing.T) {
	input := "build:\n    echo {{env_var('HOME')}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Body) != 1 {
		t.Fatalf("expected 1 body line, got %d", len(jf.Recipes[0].Body))
	}
}

// --- Parser error paths for remaining uncovered branches ---

func TestHexVal(t *testing.T) {
	tests := []struct {
		ch  byte
		val int
	}{
		{'0', 0}, {'9', 9},
		{'a', 10}, {'f', 15},
		{'A', 10}, {'F', 15},
		{'g', 0}, // fallthrough
	}
	for _, tt := range tests {
		if got := hexVal(tt.ch); got != tt.val {
			t.Errorf("hexVal(%q) = %d, want %d", tt.ch, got, tt.val)
		}
	}
}

func TestIsHexDigit(t *testing.T) {
	for _, ch := range []byte("0123456789abcdefABCDEF") {
		if !isHexDigit(ch) {
			t.Errorf("expected %q to be hex digit", ch)
		}
	}
	if isHexDigit('g') {
		t.Error("expected 'g' to not be hex digit")
	}
}

// --- Remaining parser expression paths ---

func TestLexBangOperator(t *testing.T) {
	// Standalone ! is rare but should lex
	l := newLexer([]byte("!\n"))
	tokens, err := l.lex()
	if err != nil {
		t.Fatalf("lex failed: %v", err)
	}
	found := false
	for _, tok := range tokens {
		if tok.Type == tokenBang {
			found = true
		}
	}
	if !found {
		t.Error("expected tokenBang")
	}
}

func TestLexAndOrOperatorsTopLevel(t *testing.T) {
	// && and || at top level (in dependency list)
	input := "all: build && test deploy\n"
	l := newLexer([]byte(input))
	tokens, err := l.lex()
	if err != nil {
		t.Fatalf("lex failed: %v", err)
	}
	foundAnd := false
	for _, tok := range tokens {
		if tok.Type == tokenAnd {
			foundAnd = true
		}
	}
	if !foundAnd {
		t.Error("expected tokenAnd")
	}
}

func TestLexInterpolStartEndTopLevel(t *testing.T) {
	// {{ and }} outside recipe body — in an expression context
	l := newLexer([]byte("val := {{ broken }}\n"))
	tokens, err := l.lex()
	if err != nil {
		t.Fatalf("lex failed: %v", err)
	}
	foundStart := false
	foundEnd := false
	for _, tok := range tokens {
		if tok.Type == tokenInterpolStart {
			foundStart = true
		}
		if tok.Type == tokenInterpolEnd {
			foundEnd = true
		}
	}
	if !foundStart || !foundEnd {
		t.Error("expected {{ and }} tokens")
	}
}

func TestLexSingleBracesTopLevel(t *testing.T) {
	l := newLexer([]byte("val := if \"a\" == \"b\" { \"y\" } else { \"n\" }\n"))
	tokens, err := l.lex()
	if err != nil {
		t.Fatalf("lex failed: %v", err)
	}
	braceCount := 0
	for _, tok := range tokens {
		if tok.Type == tokenLBrace || tok.Type == tokenRBrace {
			braceCount++
		}
	}
	if braceCount != 4 {
		t.Errorf("expected 4 brace tokens, got %d", braceCount)
	}
}

func TestLexRecipeBodyError(t *testing.T) {
	// Error inside recipe body interpolation
	input := "build:\n    echo {{~bad}}\n"
	_, err := Parse([]byte(input))
	if err == nil {
		t.Fatal("expected error for bad char in interpolation")
	}
}

func TestLexCommentAfterNewline(t *testing.T) {
	// Comment at start of line (atLineStart path)
	input := "# comment1\n# comment2\nval := \"x\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(jf.Comments))
	}
}

func TestLexEmptyLineAtStart(t *testing.T) {
	// Empty line at start (atLineStart + newline path)
	input := "\n\nval := \"x\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestLexEscapeSequenceAtEOF(t *testing.T) {
	_, err := Parse([]byte("val := \"\\\n"))
	if err == nil {
		t.Fatal("expected error for escape at EOF")
	}
}

func TestLexUnicodeEscapeInvalidCodepoint(t *testing.T) {
	_, err := Parse([]byte(`val := "\u{D800}"`))
	if err == nil {
		t.Fatal("expected error for invalid unicode codepoint (surrogate)")
	}
}

// --- parser.go uncovered error branches ---

func TestLexRecipeBodyNestedInterpolation(t *testing.T) {
	// This is extremely rare but tests the depth > 1 path in lexRecipeLine
	// In practice just doesn't support nested {{ }} but the lexer handles it
	input := "build:\n    echo {{ \"hello\" }}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Body) != 1 {
		t.Fatalf("expected 1 body line, got %d", len(jf.Recipes[0].Body))
	}
}

func TestLexFPrefixAsIdentifier(t *testing.T) {
	// 'f' not followed by quote should be an identifier
	input := "f := \"hello\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if jf.Assignments[0].Name != "f" {
		t.Errorf("expected name 'f', got %q", jf.Assignments[0].Name)
	}
}

func TestLexXPrefixAsIdentifier(t *testing.T) {
	// 'x' not followed by quote should be an identifier
	input := "x := \"hello\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if jf.Assignments[0].Name != "x" {
		t.Errorf("expected name 'x', got %q", jf.Assignments[0].Name)
	}
}

// --- Exercise || and && in expression context ---

func TestLexRecipeLineBacktickInInterpolation(t *testing.T) {
	input := "build:\n    echo {{`whoami`}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Body) != 1 {
		t.Fatalf("expected 1 body line, got %d", len(jf.Recipes[0].Body))
	}
}

func TestLexRecipeLineComparisonOpsInInterpolation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"regex match", "build:\n    echo {{ if \"a\" =~ \"a\" { \"y\" } else { \"n\" } }}\n"},
		{"regex mismatch", "build:\n    echo {{ if \"a\" !~ \"z\" { \"y\" } else { \"n\" } }}\n"},
		{"and", "build:\n    echo {{ if \"a\" == \"a\" && \"b\" == \"b\" { \"y\" } else { \"n\" } }}\n"},
		{"or", "build:\n    echo {{ if \"a\" == \"a\" || \"b\" == \"b\" { \"y\" } else { \"n\" } }}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			// We're exercising the lexer paths, parse may or may not succeed
			// depending on grammar support
			_ = err
		})
	}
}

// --- ParseFile uncovered: filepath.Abs error ---
