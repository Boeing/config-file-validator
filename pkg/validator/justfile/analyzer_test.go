package gojust

import (
	"strings"
	"testing"
)

func TestValidateAllowDuplicateRecipes(t *testing.T) {
	input := "set allow-duplicate-recipes\n\nbuild:\n    echo a\nbuild:\n    echo b\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "recipe") && strings.Contains(d.Message, "multiple times") {
			t.Error("should not report duplicate recipe when allow-duplicate-recipes is set")
		}
	}
}

func TestValidateAllowDuplicateVariables(t *testing.T) {
	input := "set allow-duplicate-variables\n\nx := \"a\"\nx := \"b\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "variable") && strings.Contains(d.Message, "multiple times") {
			t.Error("should not report duplicate variable when allow-duplicate-variables is set")
		}
	}
}

// --- circular dependency detection ---

func TestValidateCircularDependency(t *testing.T) {
	input := "a: b\n    echo a\nb: a\n    echo b\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "circular") {
			found = true
		}
	}
	if !found {
		t.Error("expected circular dependency diagnostic")
	}
}

func TestValidateCircularDependencyThreeWay(t *testing.T) {
	input := "a: b\n    echo a\nb: c\n    echo b\nc: a\n    echo c\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "circular") {
			found = true
		}
	}
	if !found {
		t.Error("expected circular dependency diagnostic")
	}
}

func TestValidateNonCircularDeps(t *testing.T) {
	input := "a: b\n    echo a\nb: c\n    echo b\nc:\n    echo c\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "circular") {
			t.Errorf("unexpected circular dependency diagnostic: %s", d.Message)
		}
	}
}

// --- {{{{ escape ---

func TestValidateUndefinedVariable(t *testing.T) {
	input := "val := undefined_var + \"suffix\"\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined variable 'undefined_var'") {
			found = true
		}
	}
	if !found {
		t.Error("expected undefined variable diagnostic")
	}
}

func TestValidateDefinedVariable(t *testing.T) {
	input := "name := \"world\"\ngreeting := \"hello \" + name\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined variable") {
			t.Errorf("unexpected: %s", d.Message)
		}
	}
}

func TestValidateRecipeParamNotUndefined(t *testing.T) {
	input := "build target:\n    echo {{target}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined variable 'target'") {
			t.Error("recipe parameter should not be flagged as undefined")
		}
	}
}

func TestValidateUndefinedVarInRecipeBody(t *testing.T) {
	input := "build:\n    echo {{missing}}\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined variable 'missing'") {
			found = true
		}
	}
	if !found {
		t.Error("expected undefined variable diagnostic in recipe body")
	}
}

func TestValidateBuiltinFunction(t *testing.T) {
	input := "val := arch() + \"/\" + os()\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined function") {
			t.Errorf("builtin function should not be flagged: %s", d.Message)
		}
	}
}

func TestValidateUndefinedFunction(t *testing.T) {
	input := "val := not_a_function(\"arg\")\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined function 'not_a_function'") {
			found = true
		}
	}
	if !found {
		t.Error("expected undefined function diagnostic")
	}
}

func TestValidateUserDefinedFunction(t *testing.T) {
	input := "myfunc(x) := x + \"!\"\nval := myfunc(\"hello\")\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined function 'myfunc'") {
			t.Error("user-defined function should not be flagged as undefined")
		}
	}
}

func TestValidateUndefinedVarInDepArgs(t *testing.T) {
	input := "build:\n    echo done\ndeploy: (build missing_var)\n    echo deploying\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined variable 'missing_var'") {
			found = true
		}
	}
	if !found {
		t.Error("expected undefined variable in dependency args")
	}
}

func TestValidateExportUnexportConflict(t *testing.T) {
	input := "export FOO := \"bar\"\nunexport FOO\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "both exported") && strings.Contains(d.Message, "unexported") {
			found = true
		}
	}
	if !found {
		t.Error("expected export/unexport conflict diagnostic")
	}
}

func TestValidateUnexportWithoutExport(t *testing.T) {
	input := "FOO := \"bar\"\nunexport FOO\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	for _, d := range diags {
		if strings.Contains(d.Message, "both exported") {
			t.Error("should not flag non-exported variable")
		}
	}
}
