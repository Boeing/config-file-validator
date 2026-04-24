//go:build justdump

package gojust

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// justPinnedVersion is the version of just that the dump tests are validated
// against. Updating this is an intentional act — see the README.
const justPinnedVersion = "1.49.0"

// findJust locates the just binary. It checks JUST_PATH env var first,
// then falls back to PATH lookup.
func findJust() string {
	if p := os.Getenv("JUST_PATH"); p != "" {
		return p
	}
	if p, err := exec.LookPath("just"); err == nil {
		return p
	}
	return ""
}

// justInfo holds the resolved just binary path and version for the test run.
type justInfo struct {
	path    string
	version string // e.g. "1.49.0"
	pinned  bool   // true if version matches justPinnedVersion
}

func getJustInfo(t *testing.T) justInfo {
	t.Helper()
	path := findJust()
	if path == "" {
		t.Skip("just not found (set JUST_PATH or add to PATH)")
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		t.Skipf("just --version failed: %v", err)
	}
	// "just 1.49.0" -> "1.49.0"
	version := strings.TrimPrefix(strings.TrimSpace(string(out)), "just ")
	return justInfo{
		path:    path,
		version: version,
		pinned:  version == justPinnedVersion,
	}
}

func TestJustVersionPinned(t *testing.T) {
	info := getJustInfo(t)
	if info.pinned {
		t.Logf("just %s (pinned ✓)", info.version)
		return
	}
	// In strict mode (CI), fail hard on version mismatch.
	if os.Getenv("JUST_TEST_STRICT") == "1" {
		t.Fatalf("just version mismatch: got %s, pinned to %s — update justPinnedVersion or install the pinned version", info.version, justPinnedVersion)
	}
	t.Logf("just %s (pinned to %s — diffs may be version differences, not bugs)", info.version, justPinnedVersion)
}

func TestDumpCompat(t *testing.T) {
	info := getJustInfo(t)
	strict := info.pinned || os.Getenv("JUST_TEST_STRICT") == "1"

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	if !info.pinned {
		t.Logf("running against just %s (pinned to %s) — failures may be version differences", info.version, justPinnedVersion)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".just") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join("testdata", e.Name())
			absPath, _ := filepath.Abs(path)

			justOut, err := exec.Command(info.path, "--dump", "--dump-format", "json", "--justfile", absPath).CombinedOutput()
			if err != nil {
				t.Skipf("just cannot dump %s: %s", e.Name(), string(justOut))
			}

			var expected interface{}
			if err := json.Unmarshal(justOut, &expected); err != nil {
				t.Fatalf("failed to parse just output: %v", err)
			}

			jf, err := ParseFile(absPath)
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			diffs := compareDump(t, jf, expected)
			reportDiffs(t, diffs, strict)
		})
	}
}

func TestDumpCompatWithImports(t *testing.T) {
	info := getJustInfo(t)
	strict := info.pinned || os.Getenv("JUST_TEST_STRICT") == "1"

	path, _ := filepath.Abs(filepath.Join("testdata", "with_imports.just"))

	justOut, err := exec.Command(info.path, "--dump", "--dump-format", "json", "--justfile", path).CombinedOutput()
	if err != nil {
		t.Skipf("just cannot dump: %s", string(justOut))
	}

	var expected interface{}
	if err := json.Unmarshal(justOut, &expected); err != nil {
		t.Fatalf("failed to parse just output: %v", err)
	}

	jf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	diffs := compareDump(t, jf, expected)
	reportDiffs(t, diffs, strict)
}

// TestDumpExpressionEncoding verifies specific expression encodings match just's format.
func TestDumpExpressionEncoding(t *testing.T) {
	tests := []struct {
		name string
		expr Expression
		want string
	}{
		{"string", &StringLiteral{Value: "hello"}, `"hello"`},
		{"variable", &Variable{Name: "foo"}, `["variable","foo"]`},
		{"concatenation", &Concatenation{
			Left:  &StringLiteral{Value: "a"},
			Right: &Variable{Name: "b"},
		}, `["concatenate","a",["variable","b"]]`},
		{"path_join", &PathJoin{
			Left:  &Variable{Name: "dir"},
			Right: &StringLiteral{Value: "file"},
		}, `["join",["variable","dir"],"file"]`},
		{"function_call", &FunctionCall{
			Name:      "env_var",
			Arguments: []Expression{&StringLiteral{Value: "HOME"}},
		}, `["call","env_var","HOME"]`},
		{"function_no_args", &FunctionCall{
			Name: "arch",
		}, `["call","arch"]`},
		{"backtick", &BacktickExpr{Command: "echo hi"}, `["evaluate","echo hi"]`},
		{"conditional", &Conditional{
			Condition: Comparison{
				Left:     &StringLiteral{Value: "a"},
				Right:    &StringLiteral{Value: "b"},
				Operator: "==",
			},
			Then:      &StringLiteral{Value: "yes"},
			Otherwise: &StringLiteral{Value: "no"},
		}, `["if",["==","a","b"],"yes","no"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := json.Marshal(dumpExpression(tt.expr))
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// compareDump converts our AST to the dump format and compares against just's output.
func compareDump(t *testing.T, jf *Justfile, expected interface{}) []string {
	t.Helper()
	ours := toJustDump(jf, "")

	oursJSON, _ := json.Marshal(ours)
	var oursNorm interface{}
	json.Unmarshal(oursJSON, &oursNorm)

	skipFields := map[string]bool{"source": true}
	cleanMap(oursNorm, skipFields)
	cleanMap(expected, skipFields)

	return compareWithJustDump(oursNorm, expected, "$")
}

// reportDiffs logs or fails depending on strict mode.
// Strict mode (pinned version or JUST_TEST_STRICT=1): diffs are test failures.
// Non-strict (different just version): diffs are warnings via t.Log.
func reportDiffs(t *testing.T, diffs []string, strict bool) {
	t.Helper()
	if len(diffs) == 0 {
		return
	}

	report := t.Errorf
	if !strict {
		report = func(format string, args ...interface{}) {
			t.Logf("[WARN] "+format, args...)
		}
	}

	report("found %d differences:", len(diffs))
	max := 50
	if len(diffs) < max {
		max = len(diffs)
	}
	for _, d := range diffs[:max] {
		report("  %s", d)
	}
	if len(diffs) > 50 {
		report("  ... and %d more", len(diffs)-50)
	}
}

func cleanMap(v interface{}, skip map[string]bool) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k := range skip {
			delete(val, k)
		}
		for _, child := range val {
			cleanMap(child, skip)
		}
	case []interface{}:
		for _, child := range val {
			cleanMap(child, skip)
		}
	}
}
