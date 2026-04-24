package gojust

import (
	"testing"
)

func TestMultilineStringInAssignment(t *testing.T) {
	input := "val := 'multi\nline\nstring'\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestMultilineStringInDepArg(t *testing.T) {
	input := "build x:\n    echo done\nfoo: (build 'multi\nline\narg')\n    echo deploying\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes) != 2 {
		t.Errorf("expected 2 recipes, got %d", len(jf.Recipes))
	}
}

func TestMultilineStringInInterpolation(t *testing.T) {
	input := "build:\n    echo {{'multi\nline\nstring'}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes) != 1 {
		t.Errorf("expected 1 recipe, got %d", len(jf.Recipes))
	}
}

func TestMultilineQuotedStringInAssignment(t *testing.T) {
	input := "val := \"multi\nline\nquoted\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestMultilineStringInConditional(t *testing.T) {
	input := "val := if 'a' == 'b' { 'multi\nline\nyes' } else { 'multi\nline\nno' }\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestMultilineStringInFunctionArg(t *testing.T) {
	input := "val := replace('multi\nline\nstring', 'line', 'LINE')\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(jf.Assignments))
	}
}

func TestRecipeBodyTextIsShell(t *testing.T) {
	// Single quotes in recipe body text are shell quotes, not justfile strings.
	// The lexer should treat them as opaque text.
	input := "build:\n    echo 'hello world'\n    echo \"shell quoted\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Body) != 2 {
		t.Errorf("expected 2 body lines, got %d", len(jf.Recipes[0].Body))
	}
}
