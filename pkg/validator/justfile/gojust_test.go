package gojust

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBasicJustfile(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "basic.just"))
	if err != nil {
		t.Fatal(err)
	}

	jf, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(jf.Assignments) != 4 {
		t.Errorf("expected 4 assignments, got %d", len(jf.Assignments))
	}
	if len(jf.Recipes) != 4 {
		t.Errorf("expected 4 recipes, got %d", len(jf.Recipes))
	}
	if len(jf.Aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(jf.Aliases))
	}
	if len(jf.Settings) != 2 {
		t.Errorf("expected 2 settings, got %d", len(jf.Settings))
	}
}

func TestParseAdvancedJustfile(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "advanced.just"))
	if err != nil {
		t.Fatal(err)
	}

	jf, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Count items
	if len(jf.Settings) != 4 {
		t.Errorf("expected 4 settings, got %d", len(jf.Settings))
	}
	if len(jf.Aliases) != 3 {
		t.Errorf("expected 3 aliases, got %d", len(jf.Aliases))
	}

	// Find specific recipes
	recipeNames := make(map[string]*Recipe)
	for _, r := range jf.Recipes {
		recipeNames[r.Name] = r
	}

	// Variadic params
	if r, ok := recipeNames["test"]; ok {
		if len(r.Parameters) != 1 || r.Parameters[0].Variadic != "+" {
			t.Error("expected test recipe to have + variadic param")
		}
	} else {
		t.Error("missing recipe 'test'")
	}

	if r, ok := recipeNames["test-optional"]; ok {
		if len(r.Parameters) != 1 || r.Parameters[0].Variadic != "*" {
			t.Error("expected test-optional recipe to have * variadic param")
		}
	} else {
		t.Error("missing recipe 'test-optional'")
	}

	// Export param
	if r, ok := recipeNames["run-with-env"]; ok {
		if len(r.Parameters) != 1 || !r.Parameters[0].Export {
			t.Error("expected run-with-env to have exported param")
		}
	} else {
		t.Error("missing recipe 'run-with-env'")
	}

	// Quiet recipe
	if r, ok := recipeNames["quiet-recipe"]; ok {
		if !r.Quiet {
			t.Error("expected quiet-recipe to be quiet")
		}
	} else {
		t.Error("missing recipe 'quiet-recipe'")
	}

	// Attributes
	if r, ok := recipeNames["ci"]; ok {
		if len(r.Attributes) != 2 {
			t.Errorf("expected 2 attributes on ci, got %d", len(r.Attributes))
		}
	} else {
		t.Error("missing recipe 'ci'")
	}

	if r, ok := recipeNames["unix-only"]; ok {
		if len(r.Attributes) != 2 {
			t.Errorf("expected 2 attributes on unix-only, got %d", len(r.Attributes))
		}
	} else {
		t.Error("missing recipe 'unix-only'")
	}
}

// --- Assignment tests ---

func TestParseAssignment(t *testing.T) {
	jf, err := Parse([]byte(`name := "world"`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(jf.Assignments))
	}
	if jf.Assignments[0].Name != "name" {
		t.Errorf("expected name 'name', got %q", jf.Assignments[0].Name)
	}
}

func TestParseExportAssignment(t *testing.T) {
	jf, err := Parse([]byte(`export FOO := "bar"`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(jf.Assignments))
	}
	if !jf.Assignments[0].Export {
		t.Error("expected assignment to be exported")
	}
}

func TestParseFunctionCallAssignment(t *testing.T) {
	jf, err := Parse([]byte(`home := env_var("HOME")`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(jf.Assignments))
	}
	fc, ok := jf.Assignments[0].Value.(*FunctionCall)
	if !ok {
		t.Fatalf("expected FunctionCall, got %T", jf.Assignments[0].Value)
	}
	if fc.Name != "env_var" {
		t.Errorf("expected function name 'env_var', got %q", fc.Name)
	}
}

func TestParseConcatenation(t *testing.T) {
	jf, err := Parse([]byte(`full := "a" + "b" + "c"`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	_, ok := jf.Assignments[0].Value.(*Concatenation)
	if !ok {
		t.Fatalf("expected Concatenation, got %T", jf.Assignments[0].Value)
	}
}

func TestParsePathJoin(t *testing.T) {
	jf, err := Parse([]byte(`p := "a" / "b" / "c"`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	_, ok := jf.Assignments[0].Value.(*PathJoin)
	if !ok {
		t.Fatalf("expected PathJoin, got %T", jf.Assignments[0].Value)
	}
}

func TestParseConditional(t *testing.T) {
	input := `val := if "a" == "b" { "yes" } else { "no" }`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	cond, ok := jf.Assignments[0].Value.(*Conditional)
	if !ok {
		t.Fatalf("expected Conditional, got %T", jf.Assignments[0].Value)
	}
	if cond.Condition.Operator != "==" {
		t.Errorf("expected operator '==', got %q", cond.Condition.Operator)
	}
}

func TestParseNestedConditional(t *testing.T) {
	input := `val := if "a" == "a" { "first" } else if "b" == "b" { "second" } else { "third" }`
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	cond, ok := jf.Assignments[0].Value.(*Conditional)
	if !ok {
		t.Fatalf("expected Conditional, got %T", jf.Assignments[0].Value)
	}
	_, ok = cond.Otherwise.(*Conditional)
	if !ok {
		t.Fatalf("expected nested Conditional in else, got %T", cond.Otherwise)
	}
}

func TestParseBacktick(t *testing.T) {
	jf, err := Parse([]byte("val := `echo hello`"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	bt, ok := jf.Assignments[0].Value.(*BacktickExpr)
	if !ok {
		t.Fatalf("expected BacktickExpr, got %T", jf.Assignments[0].Value)
	}
	if bt.Command != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", bt.Command)
	}
}

// --- Recipe tests ---

func TestParseRecipeWithParams(t *testing.T) {
	jf, err := Parse([]byte("build target='all' mode=\"debug\":\n    echo done\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(jf.Recipes))
	}
	r := jf.Recipes[0]
	if r.Name != "build" {
		t.Errorf("expected recipe name 'build', got %q", r.Name)
	}
	if len(r.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(r.Parameters))
	}
}

func TestParseRecipeWithDeps(t *testing.T) {
	jf, err := Parse([]byte("deploy: build test\n    echo deploying\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(r.Dependencies))
	}
}

func TestParseRecipeWithDepArgs(t *testing.T) {
	jf, err := Parse([]byte("deploy: (build \"release\")\n    echo deploying\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(r.Dependencies))
	}
	if r.Dependencies[0].Name != "build" {
		t.Errorf("expected dep name 'build', got %q", r.Dependencies[0].Name)
	}
	if len(r.Dependencies[0].Arguments) != 1 {
		t.Errorf("expected 1 dep argument, got %d", len(r.Dependencies[0].Arguments))
	}
}

func TestParseSubsequentDeps(t *testing.T) {
	jf, err := Parse([]byte("all: build && test deploy\n    echo done\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(r.Dependencies))
	}
	if r.Dependencies[0].Subsequent {
		t.Error("first dep should not be subsequent")
	}
	if !r.Dependencies[1].Subsequent {
		t.Error("second dep should be subsequent")
	}
	if !r.Dependencies[2].Subsequent {
		t.Error("third dep should be subsequent")
	}
}

func TestParseRecipeBody(t *testing.T) {
	input := "build:\n    echo line1\n    echo line2\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Body) != 2 {
		t.Errorf("expected 2 body lines, got %d", len(r.Body))
	}
}

func TestParseRecipeBodyInterpolation(t *testing.T) {
	input := "build name:\n    echo \"hello {{name}}\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Body) != 1 {
		t.Fatalf("expected 1 body line, got %d", len(r.Body))
	}
	// Should have text, interpolation, text fragments
	if len(r.Body[0].Fragments) < 2 {
		t.Errorf("expected at least 2 fragments, got %d", len(r.Body[0].Fragments))
	}
	// Check that one fragment is an interpolation
	foundInterp := false
	for _, f := range r.Body[0].Fragments {
		if _, ok := f.(*InterpolationFragment); ok {
			foundInterp = true
		}
	}
	if !foundInterp {
		t.Error("expected to find an interpolation fragment")
	}
}

func TestParseRecipeLinePrefixes(t *testing.T) {
	input := "build:\n    @quiet\n    -error\n    @-both\n    -@also\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	r := jf.Recipes[0]
	if len(r.Body) != 4 {
		t.Fatalf("expected 4 body lines, got %d", len(r.Body))
	}
	if !r.Body[0].Quiet || r.Body[0].NoError {
		t.Error("line 0: expected quiet only")
	}
	if r.Body[1].Quiet || !r.Body[1].NoError {
		t.Error("line 1: expected error-suppressed only")
	}
	if !r.Body[2].Quiet || !r.Body[2].NoError {
		t.Error("line 2: expected both quiet and error-suppressed")
	}
	if !r.Body[3].Quiet || !r.Body[3].NoError {
		t.Error("line 3: expected both quiet and error-suppressed")
	}
}

func TestParseMultipleRecipes(t *testing.T) {
	input := "build:\n    echo build\n\ntest:\n    echo test\n\ndeploy:\n    echo deploy\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes) != 3 {
		t.Errorf("expected 3 recipes, got %d", len(jf.Recipes))
	}
}

// --- Import/Module tests ---

func TestParseImport(t *testing.T) {
	jf, err := Parse([]byte(`import 'foo/bar.just'`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(jf.Imports))
	}
	if jf.Imports[0].Path != "foo/bar.just" {
		t.Errorf("expected path 'foo/bar.just', got %q", jf.Imports[0].Path)
	}
	if jf.Imports[0].Justfile != nil {
		t.Error("expected unresolved import from Parse()")
	}
}

func TestParseOptionalImport(t *testing.T) {
	jf, err := Parse([]byte(`import? 'optional.just'`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !jf.Imports[0].Optional {
		t.Error("expected optional import")
	}
}

func TestParseModule(t *testing.T) {
	jf, err := Parse([]byte(`mod bar`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(jf.Modules))
	}
	if jf.Modules[0].Name != "bar" {
		t.Errorf("expected module name 'bar', got %q", jf.Modules[0].Name)
	}
}

func TestParseModuleWithPath(t *testing.T) {
	jf, err := Parse([]byte(`mod foo 'path/to/foo.just'`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if jf.Modules[0].Path != "path/to/foo.just" {
		t.Errorf("expected path 'path/to/foo.just', got %q", jf.Modules[0].Path)
	}
}

func TestParseOptionalModule(t *testing.T) {
	jf, err := Parse([]byte(`mod? optional`))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !jf.Modules[0].Optional {
		t.Error("expected optional module")
	}
}

func TestParseFileWithImports(t *testing.T) {
	jf, err := ParseFile(filepath.Join("testdata", "with_imports.just"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(jf.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(jf.Imports))
	}
	if jf.Imports[0].Justfile == nil {
		t.Fatal("expected resolved import")
	}
	if len(jf.Imports[0].Justfile.Recipes) != 2 {
		t.Errorf("expected 2 recipes in import, got %d", len(jf.Imports[0].Justfile.Recipes))
	}

	if len(jf.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(jf.Modules))
	}
	if jf.Modules[0].Justfile == nil {
		t.Fatal("expected resolved module")
	}
	if len(jf.Modules[0].Justfile.Recipes) != 2 {
		t.Errorf("expected 2 recipes in module, got %d", len(jf.Modules[0].Justfile.Recipes))
	}
}

// --- Setting tests ---

func TestParseSetting(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"boolean implicit", "set dotenv-load\n"},
		{"boolean explicit", "set dotenv-load := true\n"},
		{"string", "set tempdir := '/tmp'\n"},
		{"list", "set shell := ['bash', '-cu']\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if len(jf.Settings) != 1 {
				t.Errorf("expected 1 setting, got %d", len(jf.Settings))
			}
		})
	}
}

// --- Attribute tests ---

func TestParseAttribute(t *testing.T) {
	input := "[private]\nbuild:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(jf.Recipes[0].Attributes))
	}
	if jf.Recipes[0].Attributes[0].Name != "private" {
		t.Errorf("expected attribute 'private', got %q", jf.Recipes[0].Attributes[0].Name)
	}
}

func TestParseAttributeWithValue(t *testing.T) {
	input := "[group: 'ci']\nbuild:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	attr := jf.Recipes[0].Attributes[0]
	if attr.Name != "group" {
		t.Errorf("expected attribute 'group', got %q", attr.Name)
	}
	if len(attr.Arguments) != 1 || attr.Arguments[0].Value != "ci" {
		t.Errorf("expected argument 'ci', got %v", attr.Arguments)
	}
}

func TestParseAttributeWithArgs(t *testing.T) {
	input := "[confirm('are you sure?')]\ndeploy:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	attr := jf.Recipes[0].Attributes[0]
	if attr.Name != "confirm" {
		t.Errorf("expected attribute 'confirm', got %q", attr.Name)
	}
	if len(attr.Arguments) != 1 || attr.Arguments[0].Value != "are you sure?" {
		t.Errorf("expected argument 'are you sure?', got %v", attr.Arguments)
	}
}

func TestParseMultipleAttributes(t *testing.T) {
	input := "[private, group: 'ci']\nbuild:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(jf.Recipes[0].Attributes) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(jf.Recipes[0].Attributes))
	}
}

// --- String type tests ---

func TestParseStringTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"quoted", `val := "hello"`, "hello"},
		{"raw", `val := 'hello'`, "hello"},
		{"escaped newline", `val := "hello\nworld"`, "hello\nworld"},
		{"raw no escape", `val := 'hello\nworld'`, `hello\nworld`},
		{"backtick", "val := `echo hi`", "echo hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			switch v := jf.Assignments[0].Value.(type) {
			case *StringLiteral:
				if v.Value != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, v.Value)
				}
			case *BacktickExpr:
				if v.Command != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, v.Command)
				}
			default:
				t.Fatalf("unexpected type %T", v)
			}
		})
	}
}

// --- Validation tests ---

func TestValidateDuplicateRecipe(t *testing.T) {
	input := "build:\n    echo a\nbuild:\n    echo b\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	if len(diags) == 0 {
		t.Error("expected diagnostics for duplicate recipe")
	}
	if !strings.Contains(diags[0].Message, "defined multiple times") {
		t.Errorf("unexpected message: %s", diags[0].Message)
	}
}

func TestValidateDuplicateVariable(t *testing.T) {
	input := "x := \"a\"\nx := \"b\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	if len(diags) == 0 {
		t.Error("expected diagnostics for duplicate variable")
	}
}

func TestValidateUndefinedDependency(t *testing.T) {
	input := "build: nonexistent\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "undefined recipe") {
			found = true
		}
	}
	if !found {
		t.Error("expected error for undefined dependency")
	}
}

func TestValidateUndefinedAliasTarget(t *testing.T) {
	input := "alias b := nonexistent\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "undefined recipe") {
			found = true
		}
	}
	if !found {
		t.Error("expected error for undefined alias target")
	}
}

func TestValidateUnknownSetting(t *testing.T) {
	input := "set bogus-setting := true\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if d.Severity == SeverityWarning && strings.Contains(d.Message, "unknown setting") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for unknown setting")
	}
}

func TestValidateDuplicateParams(t *testing.T) {
	input := "build x x:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "duplicate parameter") {
			found = true
		}
	}
	if !found {
		t.Error("expected error for duplicate parameter")
	}
}

func TestValidateParamsAfterVariadic(t *testing.T) {
	input := "build *args extra:\n    echo done\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "after variadic") {
			found = true
		}
	}
	if !found {
		t.Error("expected error for params after variadic")
	}
}

func TestValidateNoIssues(t *testing.T) {
	input := "build:\n    echo build\ntest: build\n    echo test\nalias b := build\nset dotenv-load\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
	}
}

// --- Error tests ---

func TestParseErrorPosition(t *testing.T) {
	_, err := Parse([]byte("build\n    echo done\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Pos.Line == 0 {
		t.Error("expected non-zero line in parse error")
	}
}

func TestParseErrorUnterminatedString(t *testing.T) {
	_, err := Parse([]byte(`val := "unterminated`))
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("expected 'unterminated' in error, got: %v", err)
	}
}

func TestDiagnosticFormat(t *testing.T) {
	d := Diagnostic{
		Pos:      Position{Line: 10, Column: 5},
		Severity: SeverityError,
		Message:  "something broke",
		File:     "justfile",
	}
	s := d.String()
	if !strings.Contains(s, "justfile:10:5") {
		t.Errorf("expected position in diagnostic string, got: %s", s)
	}
	if !strings.Contains(s, "error") {
		t.Errorf("expected severity in diagnostic string, got: %s", s)
	}
}
