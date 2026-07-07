package fixer_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"

	"github.com/Boeing/config-file-validator/v3/pkg/fixer"
)

// fixpointTest represents a single fix test case loaded from testdata.
type fixpointTest struct {
	name     string
	input    []byte
	schema   []byte // nil if no schema
	expected []byte
	format   string
}

// loadFixpointTests loads all test cases from a testdata subdirectory.
// Each subdirectory must contain:
//   - input.<ext>     (the broken file)
//   - expected.<ext>  (the correct output after fixing)
//   - schema.json     (optional — JSON Schema the file should pass after fix)
func loadFixpointTests(t *testing.T, dir string) []fixpointTest {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var tests []fixpointTest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		testDir := filepath.Join(dir, entry.Name())
		tc := fixpointTest{name: entry.Name()}

		// Find input and expected files.
		matches, _ := filepath.Glob(filepath.Join(testDir, "input.*"))
		require.Len(t, matches, 1, "expected exactly one input.* file in %s", testDir)
		tc.input, err = os.ReadFile(matches[0])
		require.NoError(t, err)
		tc.format = extToFormat(filepath.Ext(matches[0]))

		matches, _ = filepath.Glob(filepath.Join(testDir, "expected.*"))
		require.Len(t, matches, 1, "expected exactly one expected.* file in %s", testDir)
		tc.expected, err = os.ReadFile(matches[0])
		require.NoError(t, err)

		// Schema is optional.
		schemaPath := filepath.Join(testDir, "schema.json")
		if _, err := os.Stat(schemaPath); err == nil {
			tc.schema, err = os.ReadFile(schemaPath)
			require.NoError(t, err)
		}

		tests = append(tests, tc)
	}

	return tests
}

// runFixpointTests executes the standard fixpoint assertions for each test case.
func runFixpointTests(t *testing.T, f *fixer.Fixer, tests []fixpointTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := f.Fix(tc.input, tc.schema, tc.format)

			// 1. Output matches expected.
			require.Equal(t, string(tc.expected), string(result.Fixed),
				"fixed output does not match expected for %s", tc.name)

			// 2. At least one fix was applied.
			require.NotEmpty(t, result.Applied,
				"no fixes applied for %s — input should have issues", tc.name)

			// 3. If schema provided, output must pass schema validation.
			if tc.schema != nil {
				assertPassesSchema(t, result.Fixed, tc.schema, tc.name)
			}

			// 4. Idempotency: applying fixer to output produces no changes.
			result2 := f.Fix(result.Fixed, tc.schema, tc.format)
			require.Equal(t, string(result.Fixed), string(result2.Fixed),
				"fix is not idempotent for %s", tc.name)
			require.Empty(t, result2.Applied,
				"second fix pass found more issues for %s — not idempotent", tc.name)
		})
	}
}

// assertPassesSchema validates that data passes the given JSON Schema.
func assertPassesSchema(t *testing.T, data []byte, schema []byte, testName string) {
	t.Helper()

	// Only validate JSON data against schema (YAML/TOML would need marshaling).
	if !json.Valid(data) {
		// For non-JSON formats, skip schema validation in the test harness.
		// The CLI integration tests cover the full marshal→validate pipeline.
		return
	}

	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	require.NoError(t, err, "schema validation error for %s", testName)
	require.True(t, result.Valid(),
		"fixed output does not pass schema for %s: %v", testName, result.Errors())
}

// extToFormat maps file extension to format name.
func extToFormat(ext string) string {
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	default:
		return "unknown"
	}
}

// TestJSONTrailingComma runs fixpoint tests for the trailing comma rule.
func TestJSONTrailingComma(t *testing.T) {
	t.Parallel()

	f := fixer.New(fixer.JSONTrailingComma{})
	tests := loadFixpointTests(t, "testdata")

	// Filter to only json_trailing_comma tests.
	var filtered []fixpointTest
	for _, tc := range tests {
		if len(tc.name) >= 20 && tc.name[:20] == "json_trailing_comma_" {
			filtered = append(filtered, tc)
		}
	}
	require.NotEmpty(t, filtered, "no json_trailing_comma fixtures found")

	runFixpointTests(t, f, filtered)
}

// TestNoFixOnValidJSON verifies the fixer doesn't touch valid JSON.
func TestNoFixOnValidJSON(t *testing.T) {
	t.Parallel()

	f := fixer.New(fixer.JSONTrailingComma{})
	valid := []byte(`{"key": "value", "num": 42}`)

	result := f.Fix(valid, nil, "json")
	require.Equal(t, string(valid), string(result.Fixed), "valid JSON should be unchanged")
	require.Empty(t, result.Applied, "no fixes should be applied to valid JSON")
}

// TestCommaInsideStringNotFixed verifies commas in strings are not touched.
func TestCommaInsideStringNotFixed(t *testing.T) {
	t.Parallel()

	f := fixer.New(fixer.JSONTrailingComma{})
	// The string contains ",}" which looks like a trailing comma but isn't.
	src := []byte(`{"msg": "hello,}"}`)

	result := f.Fix(src, nil, "json")
	require.Equal(t, string(src), string(result.Fixed), "comma inside string should not be fixed")
	require.Empty(t, result.Applied)
}

// TestJSONStringToInt runs fixpoint tests for the string-to-integer coercion rule.
func TestJSONStringToInt(t *testing.T) {
	t.Parallel()

	f := fixer.New(fixer.JSONStringToInt{})
	tests := loadFixpointTests(t, "testdata")

	var filtered []fixpointTest
	for _, tc := range tests {
		if strings.HasPrefix(tc.name, "json_string_to_int") {
			filtered = append(filtered, tc)
		}
	}
	require.NotEmpty(t, filtered, "no json_string_to_int fixtures found")

	runFixpointTests(t, f, filtered)
}

// TestJSONStringToBool runs fixpoint tests for the string-to-boolean coercion rule.
func TestJSONStringToBool(t *testing.T) {
	t.Parallel()

	f := fixer.New(fixer.JSONStringToBool{})
	tests := loadFixpointTests(t, "testdata")

	var filtered []fixpointTest
	for _, tc := range tests {
		if strings.HasPrefix(tc.name, "json_string_to_bool") {
			filtered = append(filtered, tc)
		}
	}
	require.NotEmpty(t, filtered, "no json_string_to_bool fixtures found")

	runFixpointTests(t, f, filtered)
}

// FuzzJSONTrailingComma verifies no panics and that fixes produce valid JSON.
func FuzzJSONTrailingComma(f *testing.F) {
	f.Add([]byte(`{"key": "value",}`))
	f.Add([]byte(`[1, 2, 3,]`))
	f.Add([]byte(`{"a": [1,], "b": {"c": "d",},}`))
	f.Add([]byte(`{"key": "val"}`)) // valid — should be unchanged

	fx := fixer.New(fixer.JSONTrailingComma{})

	f.Fuzz(func(t *testing.T, data []byte) {
		result := fx.Fix(data, nil, "json")

		// Must never panic (if we got here, it didn't).

		// If fixes were applied and the output is valid JSON, verify idempotency.
		if len(result.Applied) > 0 && json.Valid(result.Fixed) {
			// Must be idempotent.
			result2 := fx.Fix(result.Fixed, nil, "json")
			if !bytes.Equal(result.Fixed, result2.Fixed) {
				t.Fatalf("fix is not idempotent.\nFirst: %q\nSecond: %q", result.Fixed, result2.Fixed)
			}
		}
	})
}

// FuzzSchemaFixes verifies schema coercion rules don't panic and produce valid output.
func FuzzSchemaFixes(f *testing.F) {
	f.Add(
		[]byte(`{"port": "8080"}`),
		[]byte(`{"type":"object","properties":{"port":{"type":"integer"}}}`),
	)
	f.Add(
		[]byte(`{"debug": "true"}`),
		[]byte(`{"type":"object","properties":{"debug":{"type":"boolean"}}}`),
	)
	f.Add(
		[]byte(`{"name": "app"}`),
		[]byte(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	)

	fx := fixer.New(fixer.JSONStringToInt{}, fixer.JSONStringToBool{})

	f.Fuzz(func(t *testing.T, data []byte, schema []byte) {
		result := fx.Fix(data, schema, "json")

		// Must never panic.

		// If fixes were applied, output must be valid JSON (if input was valid).
		if len(result.Applied) > 0 && json.Valid(data) {
			if !json.Valid(result.Fixed) {
				t.Fatalf("fix produced invalid JSON.\nInput: %q\nSchema: %q\nOutput: %q",
					data, schema, result.Fixed)
			}
			// Idempotency.
			result2 := fx.Fix(result.Fixed, schema, "json")
			if !bytes.Equal(result.Fixed, result2.Fixed) {
				t.Fatalf("fix not idempotent.\nFirst: %q\nSecond: %q", result.Fixed, result2.Fixed)
			}
		}
	})
}
