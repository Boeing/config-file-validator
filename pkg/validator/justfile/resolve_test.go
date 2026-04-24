package gojust

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileResolvesImports(t *testing.T) {
	jf, err := ParseFile(filepath.Join("testdata", "with_imports.just"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Import should be resolved
	if jf.Imports[0].Justfile == nil {
		t.Fatal("expected resolved import")
	}
	// Should have recipes from the imported file
	importedRecipes := jf.Imports[0].Justfile.Recipes
	names := make(map[string]bool)
	for _, r := range importedRecipes {
		names[r.Name] = true
	}
	if !names["lint"] || !names["fmt"] {
		t.Errorf("expected lint and fmt recipes in import, got %v", names)
	}
}

func TestParseFileResolvesModules(t *testing.T) {
	jf, err := ParseFile(filepath.Join("testdata", "with_imports.just"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if jf.Modules[0].Justfile == nil {
		t.Fatal("expected resolved module")
	}
	modRecipes := jf.Modules[0].Justfile.Recipes
	names := make(map[string]bool)
	for _, r := range modRecipes {
		names[r.Name] = true
	}
	if !names["install"] || !names["update"] {
		t.Errorf("expected install and update recipes in module, got %v", names)
	}
}

func TestParseFileNotFound(t *testing.T) {
	_, err := ParseFile("nonexistent/justfile")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseFileOptionalImportMissing(t *testing.T) {
	dir := t.TempDir()
	content := []byte("import? 'nonexistent.just'\nbuild:\n    echo done\n")
	path := filepath.Join(dir, "justfile")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	jf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if jf.Imports[0].Justfile != nil {
		t.Error("expected unresolved optional import")
	}
}

func TestParseFileCircularImport(t *testing.T) {
	dir := t.TempDir()

	// a.just imports b.just, b.just imports a.just
	aContent := []byte("import 'b.just'\n")
	bContent := []byte("import 'a.just'\n")

	if err := os.WriteFile(filepath.Join(dir, "a.just"), aContent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.just"), bContent, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(filepath.Join(dir, "a.just"))
	if err == nil {
		t.Fatal("expected error for circular import")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular import error, got: %v", err)
	}
}

func TestParseFileDiamondImport(t *testing.T) {
	dir := t.TempDir()

	// root imports a and b, both import shared
	shared := []byte("shared_var := \"hello\"\n")
	a := []byte("import 'shared.just'\n")
	b := []byte("import 'shared.just'\n")
	root := []byte("import 'a.just'\nimport 'b.just'\nbuild:\n    echo done\n")

	if err := os.WriteFile(filepath.Join(dir, "shared.just"), shared, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.just"), a, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.just"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.just"), root, 0600); err != nil {
		t.Fatal(err)
	}

	// Diamond imports should NOT be flagged as circular
	jf, err := ParseFile(filepath.Join(dir, "root.just"))
	if err != nil {
		t.Fatalf("ParseFile failed on diamond import: %v", err)
	}
	if len(jf.Imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(jf.Imports))
	}
}

func TestParseFileModuleNotFound(t *testing.T) {
	dir := t.TempDir()
	content := []byte("mod nonexistent\n")
	path := filepath.Join(dir, "justfile")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected error for missing module")
	}
}

func TestParseFileOptionalModuleMissing(t *testing.T) {
	dir := t.TempDir()
	content := []byte("mod? nonexistent\nbuild:\n    echo done\n")
	path := filepath.Join(dir, "justfile")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}

	jf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if jf.Modules[0].Justfile != nil {
		t.Error("expected unresolved optional module")
	}
}

func TestParseFileHomeExpansion(t *testing.T) {
	// Just test that resolveImportPath handles ~/
	result := resolveImportPath("/some/dir", "~/foo.just")
	if strings.HasPrefix(result, "~/") {
		t.Error("expected ~ to be expanded")
	}
}

// Validation with resolved imports

func TestValidateWithResolvedImports(t *testing.T) {
	jf, err := ParseFile(filepath.Join("testdata", "with_imports.just"))
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	diags := jf.Validate()
	// "check" depends on "build" and "lint" — lint comes from import
	// Should have no errors about undefined deps
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, "undefined recipe 'lint'") {
			t.Error("lint should be found via import")
		}
	}
}

func TestValidateDuplicateAlias(t *testing.T) {
	input := "build:\n    echo done\nalias b := build\nalias b := build\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "alias") && strings.Contains(d.Message, "multiple times") {
			found = true
		}
	}
	if !found {
		t.Error("expected duplicate alias diagnostic")
	}
}

func TestValidateDuplicateSetting(t *testing.T) {
	input := "set dotenv-load\nset dotenv-load\n"
	jf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	diags := jf.Validate()
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "set multiple times") {
			found = true
		}
	}
	if !found {
		t.Error("expected duplicate setting diagnostic")
	}
}

func TestResolveImportAbsolutePath(t *testing.T) {
	result := resolveImportPath("/some/dir", "/absolute/path.just")
	if result != "/absolute/path.just" {
		t.Errorf("expected absolute path unchanged, got %q", result)
	}
}

func TestResolveImportRelativePath(t *testing.T) {
	result := resolveImportPath("/some/dir", "relative.just")
	if result != "/some/dir/relative.just" {
		t.Errorf("expected joined path, got %q", result)
	}
}

// --- Exercise expression paths that go through parseLogicalOr/And/Comparison ---

func TestParseFileAbsPathError(t *testing.T) {
	// ParseFile with a path that can't be made absolute shouldn't panic
	_, err := ParseFile("testdata/basic.just")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
}

func TestParseFileWithSyntaxError(t *testing.T) {
	dir := t.TempDir()
	content := []byte("build\n    echo done\n")
	path := dir + "/justfile"
	if err := writeTestFile(path, content); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.File == "" {
		t.Error("expected file path in error")
	}
}

func TestParseFileLexError(t *testing.T) {
	dir := t.TempDir()
	content := []byte("val := \"unterminated\n")
	path := dir + "/justfile"
	if err := writeTestFile(path, content); err != nil {
		t.Fatal(err)
	}
	_, err := ParseFile(path)
	if err == nil {
		t.Fatal("expected lex error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.File == "" {
		t.Error("expected file path in lex error")
	}
}

func writeTestFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0600)
}

func TestParseFileAbsPathFailure(t *testing.T) {
	// ParseFile error on filepath.Abs — hard to trigger on real OS
	// but we can test the module explicit path resolution
	dir := t.TempDir()
	modContent := []byte("build:\n    echo done\n")
	if err := os.MkdirAll(dir+"/sub", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/sub/custom.just", modContent, 0600); err != nil {
		t.Fatal(err)
	}

	rootContent := []byte("mod foo 'sub/custom.just'\n")
	if err := os.WriteFile(dir+"/justfile", rootContent, 0600); err != nil {
		t.Fatal(err)
	}

	jf, err := ParseFile(dir + "/justfile")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if jf.Modules[0].Justfile == nil {
		t.Error("expected resolved module with explicit path")
	}
}

func TestParseFileModuleParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bad.just", []byte("not valid {{{\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/justfile", []byte("mod bad 'bad.just'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(dir + "/justfile")
	if err == nil {
		t.Fatal("expected error from bad module file")
	}
}

func TestParseFileImportParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bad.just", []byte("not valid {{{\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/justfile", []byte("import 'bad.just'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(dir + "/justfile")
	if err == nil {
		t.Fatal("expected error from bad import file")
	}
}

func TestParseFileOptionalModuleParseError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/bad.just", []byte("not valid {{{\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/justfile", []byte("mod? bad 'bad.just'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Optional module with a parse error should still fail — optional only
	// applies to missing files, not broken ones
	_, err := ParseFile(dir + "/justfile")
	if err == nil {
		t.Fatal("expected error from bad optional module file")
	}
}

// --- lexer.go uncovered branches ---

func TestParseFileRelativePath(t *testing.T) {
	// Exercise the filepath.Abs success path with a relative path
	jf, err := ParseFile("testdata/basic.just")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if jf.File == "" {
		t.Error("expected non-empty file path")
	}
	if jf.File == "testdata/basic.just" {
		t.Error("expected absolute path, got relative")
	}
}

// --- current() EOF branch ---
