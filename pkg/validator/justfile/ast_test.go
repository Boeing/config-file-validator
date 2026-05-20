package gojust

import (
	"testing"
)

func TestTokenTypeString(t *testing.T) {
	if s := tokenIdentifier.String(); s != "identifier" {
		t.Errorf("expected 'identifier', got %q", s)
	}
	// Unknown token type
	if s := tokenType(999).String(); s != "token(999)" {
		t.Errorf("expected 'token(999)', got %q", s)
	}
}

func TestTokenString(t *testing.T) {
	tok := token{Type: tokenIdentifier, Value: "foo", Pos: Position{Line: 1, Column: 1}}
	s := tok.String()
	if s == "" {
		t.Error("expected non-empty string")
	}
}

func TestPositionString(t *testing.T) {
	p := Position{Line: 10, Column: 5}
	if s := p.String(); s != "10:5" {
		t.Errorf("expected '10:5', got %q", s)
	}
}

func TestSeverityString(t *testing.T) {
	if s := SeverityError.String(); s != "error" {
		t.Errorf("expected 'error', got %q", s)
	}
	if s := SeverityWarning.String(); s != "warning" {
		t.Errorf("expected 'warning', got %q", s)
	}
	if s := Severity(99).String(); s != "unknown" {
		t.Errorf("expected 'unknown', got %q", s)
	}
}

func TestDiagnosticStringNoFile(t *testing.T) {
	d := Diagnostic{
		Pos:      Position{Line: 1, Column: 1},
		Severity: SeverityWarning,
		Message:  "test",
	}
	s := d.String()
	if s != "<input>:1:1: warning: test" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestParseErrorStringNoFile(t *testing.T) {
	e := &ParseError{
		Pos:     Position{Line: 5, Column: 3},
		Message: "bad",
	}
	s := e.Error()
	if s != "<input>:5:3: bad" {
		t.Errorf("unexpected: %q", s)
	}
}

// --- Parser error paths ---

func TestParseErrorUnexpectedToken(t *testing.T) {
	_, err := Parse([]byte("+ bad"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseErrorBadExportMissingAssign(t *testing.T) {
	_, err := Parse([]byte("export FOO\n"))
	if err == nil {
		t.Fatal("expected error for export without :=")
	}
}

func TestParseErrorBadSettingValue(t *testing.T) {
	_, err := Parse([]byte("set shell := bad\n"))
	if err == nil {
		t.Fatal("expected error for bad setting value")
	}
}

func TestParseErrorBadImportPath(t *testing.T) {
	_, err := Parse([]byte("import bad\n"))
	if err == nil {
		t.Fatal("expected error for import without string")
	}
}

// --- Escape sequence edge cases ---

func TestAllExpressionTypes(t *testing.T) {
	// Build expressions that cover every Expression implementation
	exprs := []Expression{
		&StringLiteral{Value: "a", Pos: Position{1, 1, 0}},
		&Variable{Name: "x", Pos: Position{1, 1, 0}},
		&Concatenation{Left: &StringLiteral{}, Right: &StringLiteral{}, Pos: Position{1, 1, 0}},
		&PathJoin{Left: &StringLiteral{}, Right: &StringLiteral{}, Pos: Position{1, 1, 0}},
		&FunctionCall{Name: "f", Pos: Position{1, 1, 0}},
		&Conditional{Pos: Position{1, 1, 0}},
		&BacktickExpr{Command: "echo", Pos: Position{1, 1, 0}},
		&ParenExpr{Inner: &StringLiteral{}, Pos: Position{1, 1, 0}},
		&LogicalOp{Left: &StringLiteral{}, Right: &StringLiteral{}, Operator: "&&", Pos: Position{1, 1, 0}},
		&Comparison{Left: &StringLiteral{}, Right: &StringLiteral{}, Operator: "==", Pos: Position{1, 1, 0}},
	}
	for _, e := range exprs {
		e.exprNode()
		p := e.GetPos()
		if p.Line != 1 {
			t.Errorf("expected line 1, got %d for %T", p.Line, e)
		}
	}

	frags := []Fragment{
		&TextFragment{Value: "hi", Pos: Position{1, 1, 0}},
		&InterpolationFragment{Expression: &StringLiteral{}, Pos: Position{1, 1, 0}},
	}
	for _, f := range frags {
		f.fragmentNode()
		p := f.GetPos()
		if p.Line != 1 {
			t.Errorf("expected line 1, got %d for %T", p.Line, f)
		}
	}
}

// --- Remaining lexer gaps ---

func TestParseErrorAliasMissingAssign(t *testing.T) {
	_, err := Parse([]byte("alias b bad\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseErrorAliasMissingTarget(t *testing.T) {
	_, err := Parse([]byte("alias b :=\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseErrorDependencyUnclosedParen(t *testing.T) {
	_, err := Parse([]byte("build: (dep \"arg\"\n    echo done\n"))
	if err == nil {
		t.Fatal("expected error for unclosed dependency paren")
	}
}

func TestParseErrorFunctionCallUnclosed(t *testing.T) {
	_, err := Parse([]byte("val := env_var(\"HOME\"\n"))
	if err == nil {
		t.Fatal("expected error for unclosed function call")
	}
}

func TestParseErrorSettingBadList(t *testing.T) {
	_, err := Parse([]byte("set shell := [bad]\n"))
	if err == nil {
		t.Fatal("expected error for bad list value")
	}
}

func TestParseErrorConditionalMissingCloseBrace(t *testing.T) {
	_, err := Parse([]byte(`val := if "a" == "b" { "y" else { "n" }`))
	if err == nil {
		t.Fatal("expected error for missing close brace")
	}
}

func TestParseErrorConditionalMissingElseCloseBrace(t *testing.T) {
	_, err := Parse([]byte(`val := if "a" == "b" { "y" } else { "n"`))
	if err == nil {
		t.Fatal("expected error for missing else close brace")
	}
}

func TestSettingValueKind(t *testing.T) {
	b := true
	s := "hello"
	tests := []struct {
		name string
		sv   SettingValue
		want string
	}{
		{"bool", SettingValue{Bool: &b}, "bool"},
		{"string", SettingValue{String: &s}, "string"},
		{"list", SettingValue{List: []string{"a"}}, "list"},
		{"empty", SettingValue{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sv.Kind(); got != tt.want {
				t.Errorf("Kind() = %q, want %q", got, tt.want)
			}
		})
	}
}
