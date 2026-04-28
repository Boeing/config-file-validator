package validator

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// xsNamespace is the W3C XML Schema namespace URI.
// Extracted as a constant to avoid DevSkim DS137138 false positives
// in test string literals (this is a namespace identifier, not a fetched URL).
const xsNamespace = "http://www.w3.org/2001/XMLSchema" // DevSkim: ignore DS137138

var (
	validPlistBytes = []byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
	<plist version="1.0">
	<dict>
		<key>CFBundleShortVersionString</key>
		<string>1.0</string>
		<key>CFBundleVersion</key>
		<string>1</string>
		<key>NSAppTransportSecurity</key>
		<dict>
			<key>NSAllowsArbitraryLoads</key>
			<true/>
		</dict>
	</dict>
	</plist>`)

	invalidPlistBytes = []byte(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
	<plist version="1.0">
	<dict>
		<key>CFBundleShortVersionString</key>
		<string>1.0</string>
		<key>CFBundleVersion</key>
		<string>1</string>
		<key>NSAppTransporT-Security</key> <!-- The hyphen in the key name here is invalid -->
		<dict>
			<key>NSAllowsArbitraryLoads</key>
		</dict> <!-- Missing value for the key 'NSAllowsArbitraryLoads' -->
	</dict>
	</plist>`)
	validSarif210Bytes = []byte(`{
		"version": "2.1.0",
		"$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
		"runs": [{
			"tool": {"driver": {"name": "test", "language": "en"}},
			"results": [],
			"language": "en",
			"newlineSequences": ["\n"]
		}]
	}`)

	validSarif22Bytes = []byte(`{
		"version": "2.2",
		"$schema": "https://docs.oasis-open.org/sarif/sarif/v2.2/csd01/schemas/sarif-schema-2.2.json",
		"guid": "12345678-1234-1234-8234-123456789012",
		"runs": [{
			"tool": {"driver": {"name": "test"}},
			"results": [],
			"newlineSequences": ["\n"]
		}]
	}`)

	invalidSchemaSarifBytes = []byte(`{
		"version": "2.1.0",
		"runs": "not_an_array"
	}`)

	validSyntaxInvalidSchemaSarifBytes = []byte(`{
		"version": "2.1.0",
		"$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
		"runs": [{
			"tool": {"driver": {"name": "test"}},
			"results": []
		}]
	}`)

	fuzzbank = [][]byte{
		[]byte(`{test": "test"}`), []byte(`{"test": "test"}`),
		[]byte(`{}`), []byte(`[]`), []byte(`{]'{}}`), []byte("no_rizz"),
		[]byte(`{"hows_the_market": "full_of_crabs"}`), []byte("a: 1\nb: 2"),
		[]byte("a: b\nc: d:::::::::::::::"),
		[]byte("<test>\n</test>"), []byte("<xml\n"), []byte("name = 123__456"),
		[]byte("name = 123"), []byte(`{[Version]\nCatalog=hidden\n}`),
		[]byte(`\nCatalog hidden\n`), []byte("key=value\nkey2=${key}"),
		[]byte("key=${key}"), []byte(`key = "value"`),
		[]byte(`"key" = "value"`), []byte(`"key1" = "value1"\n"key2"="value2"`),
		[]byte(`first_name,last_name,username\nRob,Pike,rob\n`),
		[]byte(`This string has a \" in it`), validPlistBytes, invalidPlistBytes,
		[]byte(`test = [1, 2, 3]`), []byte(`test = [1, 2,, 3]`), []byte("KEY=VALUE"),
		[]byte("=TEST"), []byte("working = true"), []byte("[*.md\nworking=false"),
	}
)

// writeTestSchema writes a JSON Schema to a temp dir and returns its absolute path.
func writeTestSchema(t *testing.T) string {
	t.Helper()
	schema := `{
	"type": "object",
	"required": ["host", "port", "database"],
	"properties": {
		"host": { "type": "string" },
		"port": { "type": "integer" },
		"database": { "type": "string" }
	},
	"additionalProperties": false
}`
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.json")
	err := os.WriteFile(p, []byte(schema), 0600)
	require.NoError(t, err)
	return p
}

var testData = []struct {
	name           string
	testInput      []byte
	expectedResult bool
	validator      Validator
}{
	{"validJson", []byte(`{"test": "test"}`), true, JSONValidator{}},
	{"invalidJson", []byte(`{test": "test"}`), false, JSONValidator{}},
	{"validYaml", []byte("a: 1\nb: 2"), true, YAMLValidator{}},
	{"invalidYaml", []byte("a: b\nc: d:::::::::::::::"), false, YAMLValidator{}},
	{"validXml", []byte("<test>\n</test>"), true, XMLValidator{}},
	{"invalidXml", []byte("<xml\n"), false, XMLValidator{}},
	{"invalidToml", []byte("name = 123__456"), false, TomlValidator{}},
	{"validToml", []byte("name = 123"), true, TomlValidator{}},
	{"validIni", []byte(`{[Version]\nCatalog=hidden\n}`), true, IniValidator{}},
	{"invalidIni", []byte(`\nCatalog hidden\n`), false, IniValidator{}},
	{"validProperties", []byte("key=value\nkey2=${key}"), true, PropValidator{}},
	{"invalidProperties", []byte("key=${key}"), false, PropValidator{}},
	{"validHcl", []byte(`key = "value"`), true, HclValidator{}},
	{"invalidHcl", []byte(`"key" = "value"`), false, HclValidator{}},
	{"multipleInvalidHcl", []byte(`"key1" = "value1"\n"key2"="value2"`), false, HclValidator{}},
	{"validCSV", []byte(`first_name,last_name,username\nRob,Pike,rob\n`), true, CsvValidator{}},
	{"invalidCSV", []byte(`This string has a \" in it`), false, CsvValidator{}},
	{"validPlist", validPlistBytes, true, PlistValidator{}},
	{"invalidPlist", invalidPlistBytes, false, PlistValidator{}},
	{"validHocon", []byte(`test = [1, 2, 3]`), true, HoconValidator{}},
	{"invalidHocon", []byte(`test = [1, 2,, 3]`), false, HoconValidator{}},
	{"validEnv", []byte("KEY=VALUE"), true, EnvValidator{}},
	{"invalidEnv", []byte("=TEST"), false, EnvValidator{}},
	{"validEditorConfig", []byte("working = true"), true, EditorConfigValidator{}},
	{"invalidEditorConfig", []byte("[*.md\nworking=false"), false, EditorConfigValidator{}},
	{"validToon", []byte("users[2]{id,name,role}:\n  1,Alice,admin\n  2,Bob,user\n"), true, ToonValidator{}},
	{"invalidToon", []byte("users2]{id,name,role}:\n  1,Alice,admin\n  2,Bob,user\n"), false, ToonValidator{}},
	{"validJustfile", []byte("default:\n    echo hello\n"), true, JustfileValidator{}},
	{"validJustfileParams", []byte("build target=\"linux\":\n    GOOS={{target}} go build\n"), true, JustfileValidator{}},
	{"invalidJustfileUnterminatedString", []byte("name := \"hello\n"), false, JustfileValidator{}},
	{"validSarif210", validSarif210Bytes, true, SarifValidator{}},
	{"validSarif22", validSarif22Bytes, true, SarifValidator{}},
	{"invalidSarif", []byte(`{"not": "sarif"}`), false, SarifValidator{}},
	{"validJsonc", []byte("// comment\n{\"key\": \"value\"}"), true, JSONCValidator{}},
	{"validJsoncBlockComment", []byte("/* block */\n{\"key\": \"value\"}"), true, JSONCValidator{}},
	{"validJsoncTrailingComma", []byte(`{"a": 1, "b": 2,}`), true, JSONCValidator{}},
	{"invalidJsonc", []byte(`{"bad": }`), false, JSONCValidator{}},
	{"validJsoncNoComments", []byte(`{"key": "value"}`), true, JSONCValidator{}},
	{"validKdl", []byte("name \"Bob\"\nage 76\nactive true\n"), true, KdlValidator{}},
	{"validKdlChildren", []byte("package {\n    name \"foo\"\n    version \"1.0\"\n}\n"), true, KdlValidator{}},
	{"invalidKdlUnterminatedString", []byte("name \"Bob\n"), false, KdlValidator{}},
	{"invalidKdlUnclosedChildren", []byte("package {\n    name \"foo\"\n"), false, KdlValidator{}},
}

func Test_ValidationInput(t *testing.T) {
	t.Parallel()

	for _, tcase := range testData {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			valid, err := tcase.validator.ValidateSyntax(tcase.testInput)
			if valid != tcase.expectedResult {
				t.Errorf("incorrect result: expected %v, got %v", tcase.expectedResult, valid)
			}

			if valid && err != nil {
				t.Error("incorrect result: err was not nil", err)
			}

			if !valid && err == nil {
				t.Error("incorrect result: function returned a nil error")
			}
		})
	}
}

func addFuzzCases(f *testing.F) {
	f.Helper()
	for _, tc := range fuzzbank {
		f.Add(tc)
	}
}

func fuzzFunction(v Validator) func(*testing.T, []byte) {
	return func(_ *testing.T, a []byte) {
		_, _ = v.ValidateSyntax(a)
	}
}

func FuzzJsonValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(JSONValidator{}))
}

func FuzzYamlValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(YAMLValidator{}))
}

func FuzzXMLValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(XMLValidator{}))
}

func FuzzTomlValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(TomlValidator{}))
}

func FuzzIniValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(IniValidator{}))
}

func FuzzPropValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(PropValidator{}))
}

func FuzzHclValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(HclValidator{}))
}

func FuzzCsvValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(CsvValidator{}))
}

func FuzzPlistValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(PlistValidator{}))
}

func FuzzHoconValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(HoconValidator{}))
}

func FuzzEnvValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(EnvValidator{}))
}

func FuzzEditorConfigValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(EditorConfigValidator{}))
}

func FuzzToonValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(ToonValidator{}))
}

func FuzzSarifValidator(f *testing.F) {
	addFuzzCases(f)
	f.Fuzz(fuzzFunction(SarifValidator{}))
}

func Test_ValidationError(t *testing.T) {
	t.Parallel()
	inner := errors.New("something broke")
	ve := &ValidationError{Err: inner, Line: 5, Column: 10}
	require.Equal(t, "something broke", ve.Error())
	require.ErrorIs(t, ve, inner)
}

var schemaTestData = []struct {
	name           string
	testInput      []byte
	expectedResult bool
}{
	{"validSchema210", validSarif210Bytes, true},
	{"validSchema22", validSarif22Bytes, true},
	{"invalidSchema", invalidSchemaSarifBytes, false},
	{"validSyntaxInvalidSchema", validSyntaxInvalidSchemaSarifBytes, false},
	{"invalidVersion", []byte(`{"version": "9.9"}`), false},
}

func Test_SarifValidateSchema(t *testing.T) {
	t.Parallel()

	for _, tcase := range schemaTestData {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			valid, err := SarifValidator{}.ValidateSchema(tcase.testInput, "")
			if valid != tcase.expectedResult {
				t.Errorf("incorrect result: expected %v, got %v (err: %v)", tcase.expectedResult, valid, err)
			}

			if valid && err != nil {
				t.Error("incorrect result: err was not nil", err)
			}

			if !valid && err == nil {
				t.Error("incorrect result: function returned a nil error")
			}
		})
	}
}

func Test_JSONValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	// JSON without $schema should return ErrNoSchema
	valid, err := JSONValidator{}.ValidateSchema([]byte(`{"key": "value"}`), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_JSONValidateSchemaEmptySchema(t *testing.T) {
	t.Parallel()
	valid, err := JSONValidator{}.ValidateSchema([]byte(`{"$schema": "", "key": "value"}`), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must not be empty")
}

func Test_JSONValidateSchemaInvalidJSON(t *testing.T) {
	t.Parallel()
	// Invalid JSON should fail
	valid, err := JSONValidator{}.ValidateSchema([]byte(`{bad`), "")
	require.False(t, valid)
	require.Error(t, err)
}

func Test_JSONValidateSchemaArrayRoot(t *testing.T) {
	t.Parallel()
	// JSON array (not object) has no $schema — should return ErrNoSchema
	valid, err := JSONValidator{}.ValidateSchema([]byte(`[1, 2, 3]`), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_JSONValidateSchemaValid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	doc := `{"$schema": "` + schema + `", "host": "db.example.com", "port": 5432, "database": "mydb"}`
	valid, err := JSONValidator{}.ValidateSchema([]byte(doc), "")
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_JSONValidateSchemaInvalidDoc(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	doc := `{"$schema": "` + schema + `", "host": "db.example.com", "port": "not_a_number", "database": "mydb"}`
	valid, err := JSONValidator{}.ValidateSchema([]byte(doc), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_YAMLValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	valid, err := YAMLValidator{}.ValidateSchema([]byte("key: value\n"), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_YAMLValidateSchemaWithComment(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "# yaml-language-server: $schema=" + schema + "\nhost: db.example.com\nport: 5432\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_YAMLValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "# yaml-language-server: $schema=" + schema + "\nhost: db.example.com\nport: not_a_number\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_YAMLValidateSchemaCommentAfterBlank(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "\n# yaml-language-server: $schema=" + schema + "\nhost: db.example.com\nport: 5432\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_YAMLValidateSchemaCommentAfterContent(t *testing.T) {
	t.Parallel()
	// Schema comment after non-comment content should be ignored
	yaml := "key: value\n# yaml-language-server: $schema=https://example.com/schema.json\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_TomlValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	valid, err := TomlValidator{}.ValidateSchema([]byte("key = \"value\"\n"), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_TomlValidateSchemaValid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toml := `"$schema" = "` + schema + "\"\nhost = \"db.example.com\"\nport = 5432\ndatabase = \"mydb\"\n"
	valid, err := TomlValidator{}.ValidateSchema([]byte(toml), filepath.Join(filepath.Dir(schema), "test.toml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_TomlValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toml := `"$schema" = "` + schema + "\"\nhost = \"db.example.com\"\nport = \"not_a_number\"\ndatabase = \"mydb\"\n"
	valid, err := TomlValidator{}.ValidateSchema([]byte(toml), filepath.Join(filepath.Dir(schema), "test.toml"))
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_ToonValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	valid, err := ToonValidator{}.ValidateSchema([]byte("key: value\n"), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_ToonValidateSchemaValid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toonDoc := "\"$schema\": " + schema + "\nhost: db.example.com\nport: 5432\ndatabase: mydb\n"
	valid, err := ToonValidator{}.ValidateSchema([]byte(toonDoc), filepath.Join(filepath.Dir(schema), "test.toon"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_ToonValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toonDoc := "\"$schema\": " + schema + "\nhost: db.example.com\nport: \"not_a_number\"\ndatabase: mydb\n"
	valid, err := ToonValidator{}.ValidateSchema([]byte(toonDoc), filepath.Join(filepath.Dir(schema), "test.toon"))
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_ToonValidateSchemaNotObject(t *testing.T) {
	t.Parallel()
	// TOON that decodes to a non-object should return ErrNoSchema
	valid, err := ToonValidator{}.ValidateSchema([]byte("items[3]: 1, 2, 3\n"), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_extractYAMLSchemaComment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard", "# yaml-language-server: $schema=https://example.com/s.json\nkey: val", "https://example.com/s.json"},
		{"with spaces", "#  yaml-language-server:  $schema=https://example.com/s.json \nkey: val", "https://example.com/s.json"},
		{"blank lines before", "\n\n# yaml-language-server: $schema=https://example.com/s.json\nkey: val", "https://example.com/s.json"},
		{"no comment", "key: val", ""},
		{"wrong comment", "# just a comment\nkey: val", ""},
		{"after content", "key: val\n# yaml-language-server: $schema=https://example.com/s.json", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractYAMLSchemaComment([]byte(tc.input))
			require.Equal(t, tc.expected, got)
		})
	}
}

// --- MarshalToJSON tests ---

func Test_JSONMarshalToJSON(t *testing.T) {
	t.Parallel()
	out, err := JSONValidator{}.MarshalToJSON([]byte(`{"$schema": "x", "key": "val"}`))
	require.NoError(t, err)
	require.NotContains(t, string(out), "$schema")
	require.Contains(t, string(out), "key")
}

func Test_JSONMarshalToJSONArray(t *testing.T) {
	t.Parallel()
	out, err := JSONValidator{}.MarshalToJSON([]byte(`[1,2,3]`))
	require.NoError(t, err)
	require.Equal(t, "[1,2,3]", string(out))
}

func Test_JSONMarshalToJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := JSONValidator{}.MarshalToJSON([]byte(`{bad`))
	require.Error(t, err)
}

func Test_YAMLMarshalToJSON(t *testing.T) {
	t.Parallel()
	out, err := YAMLValidator{}.MarshalToJSON([]byte("key: value\nnum: 42\n"))
	require.NoError(t, err)
	require.Contains(t, string(out), `"key":`)
	require.Contains(t, string(out), `"num":`)
}

func Test_YAMLMarshalToJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := YAMLValidator{}.MarshalToJSON([]byte("a: b\nc: d:::::::::::::::"))
	require.Error(t, err)
}

func Test_TomlMarshalToJSON(t *testing.T) {
	t.Parallel()
	out, err := TomlValidator{}.MarshalToJSON([]byte("\"$schema\" = \"x\"\nkey = \"val\"\n"))
	require.NoError(t, err)
	require.NotContains(t, string(out), "$schema")
	require.Contains(t, string(out), "key")
}

func Test_TomlMarshalToJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := TomlValidator{}.MarshalToJSON([]byte("key = 123__456"))
	require.Error(t, err)
}

func Test_ToonMarshalToJSON(t *testing.T) {
	t.Parallel()
	out, err := ToonValidator{}.MarshalToJSON([]byte("\"$schema\": x\nkey: val\n"))
	require.NoError(t, err)
	require.NotContains(t, string(out), "$schema")
	require.Contains(t, string(out), "key")
}

func Test_ToonMarshalToJSONNonObject(t *testing.T) {
	t.Parallel()
	out, err := ToonValidator{}.MarshalToJSON([]byte("items[3]: 1, 2, 3\n"))
	require.NoError(t, err)
	require.NotNil(t, out)
}

func Test_ToonMarshalToJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := ToonValidator{}.MarshalToJSON([]byte("users2]{id}:\n  1,Alice\n"))
	require.Error(t, err)
}

// --- resolveSchemaURL tests ---

func Test_resolveSchemaURLHTTPS(t *testing.T) {
	t.Parallel()
	got := resolveSchemaURL("https://example.com/schema.json", "/some/file.json")
	require.Equal(t, "https://example.com/schema.json", got)
}

func Test_resolveSchemaURLAbsPath(t *testing.T) {
	t.Parallel()
	got := resolveSchemaURL("/opt/schemas/schema.json", "/some/file.json")
	require.Equal(t, "file:///opt/schemas/schema.json", got)
}

func Test_resolveSchemaURLRelative(t *testing.T) {
	t.Parallel()
	got := resolveSchemaURL("schema.json", "/project/config/file.json")
	require.Equal(t, "file:///project/config/schema.json", got)
}

// --- ValidateSchema edge cases ---

func Test_JSONValidateSchemaNonStringSchema(t *testing.T) {
	t.Parallel()
	valid, err := JSONValidator{}.ValidateSchema([]byte(`{"$schema": 123}`), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must be a string")
}

func Test_TomlValidateSchemaEmptySchema(t *testing.T) {
	t.Parallel()
	valid, err := TomlValidator{}.ValidateSchema([]byte("\"$schema\" = \"\"\nkey = \"val\"\n"), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must not be empty")
}

func Test_TomlValidateSchemaInvalidToml(t *testing.T) {
	t.Parallel()
	valid, err := TomlValidator{}.ValidateSchema([]byte("key = 123__456"), "")
	require.False(t, valid)
	require.Error(t, err)
}

func Test_ToonValidateSchemaEmptySchema(t *testing.T) {
	t.Parallel()
	valid, err := ToonValidator{}.ValidateSchema([]byte("\"$schema\": \"\"\nkey: val\n"), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must not be empty")
}

func Test_ToonValidateSchemaInvalidToon(t *testing.T) {
	t.Parallel()
	valid, err := ToonValidator{}.ValidateSchema([]byte("users2]{id}:\n  1,Alice\n"), "")
	require.False(t, valid)
	require.Error(t, err)
}

func Test_YAMLValidateSchemaInvalidYAML(t *testing.T) {
	t.Parallel()
	// Valid YAML comment with schema, but invalid YAML body
	yaml := "# yaml-language-server: $schema=https://example.com/s.json\na: b\nc: d:::::::::::::::\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), "")
	require.False(t, valid)
	require.Error(t, err)
}

// --- XML XSD validation tests ---

func Test_XMLValidateSchemaValid(t *testing.T) {
	t.Parallel()
	xsdFile := writeTestXSD(t)
	xml := `<?xml version="1.0"?>
<config xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
        xsi:noNamespaceSchemaLocation="` + xsdFile + `">
  <host>db.example.com</host>
  <port>5432</port>
</config>`
	valid, err := XMLValidator{}.ValidateSchema([]byte(xml), "")
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_XMLValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	xsdFile := writeTestXSD(t)
	xml := `<?xml version="1.0"?>
<config xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
        xsi:noNamespaceSchemaLocation="` + xsdFile + `">
  <host>db.example.com</host>
  <port>not_a_number</port>
</config>`
	valid, err := XMLValidator{}.ValidateSchema([]byte(xml), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_XMLValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?><root><key>value</key></root>`
	valid, err := XMLValidator{}.ValidateSchema([]byte(xml), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_XMLValidateSchemaBadNamespace(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<root xsi:noNamespaceSchemaLocation="schema.xsd"
      xmlns:xsi="http://www.w3.org/2001/XMLSchemainstance">
  <key>value</key>
</root>`
	valid, err := XMLValidator{}.ValidateSchema([]byte(xml), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "incorrect namespace")
}

func Test_XMLValidateSchemaRelativePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// DevSkim: ignore DS137138 -- W3C XML Schema namespace is a fixed URI
	xsdContent := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<xs:schema xmlns:xs="` + xsNamespace + `">` +
		`<xs:element name="root"><xs:complexType><xs:sequence>` +
		`<xs:element name="name" type="xs:string"/>` +
		`</xs:sequence></xs:complexType></xs:element></xs:schema>`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.xsd"), []byte(xsdContent), 0600))

	xml := `<?xml version="1.0"?>
<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:noNamespaceSchemaLocation="schema.xsd">
  <name>test</name>
</root>`
	valid, err := XMLValidator{}.ValidateSchema([]byte(xml), filepath.Join(dir, "doc.xml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_XMLValidateXSDExported(t *testing.T) {
	t.Parallel()
	xsdFile := writeTestXSD(t)
	xml := `<?xml version="1.0"?>
<config>
  <host>db.example.com</host>
  <port>5432</port>
</config>`
	valid, err := ValidateXSD([]byte(xml), xsdFile)
	require.True(t, valid)
	require.NoError(t, err)
}

// --- XML DTD validation tests ---

func Test_XMLDTDValid(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE config [
  <!ELEMENT config (host, port)>
  <!ELEMENT host (#PCDATA)>
  <!ELEMENT port (#PCDATA)>
]>
<config>
  <host>db.example.com</host>
  <port>5432</port>
</config>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_XMLDTDMissingElement(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE config [
  <!ELEMENT config (host, port)>
  <!ELEMENT host (#PCDATA)>
  <!ELEMENT port (#PCDATA)>
]>
<config>
  <host>db.example.com</host>
</config>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.False(t, valid)
	require.Error(t, err)
}

func Test_XMLDTDWrongElement(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE config [
  <!ELEMENT config (host, port)>
  <!ELEMENT host (#PCDATA)>
  <!ELEMENT port (#PCDATA)>
]>
<config>
  <host>db.example.com</host>
  <port>5432</port>
  <extra>not_allowed</extra>
</config>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.False(t, valid)
	require.Error(t, err)
}

func Test_XMLDTDRequiredAttribute(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE doc [
  <!ELEMENT doc EMPTY>
  <!ATTLIST doc id ID #REQUIRED>
]>
<doc id="x1"/>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_XMLDTDMissingRequiredAttribute(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE doc [
  <!ELEMENT doc EMPTY>
  <!ATTLIST doc id ID #REQUIRED>
]>
<doc/>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.False(t, valid)
	require.Error(t, err)
}

func Test_XMLDTDWrongRootElement(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<!DOCTYPE config [
  <!ELEMENT config (host)>
  <!ELEMENT host (#PCDATA)>
]>
<wrong>
  <host>db.example.com</host>
</wrong>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.False(t, valid)
	require.Error(t, err)
}

func Test_XMLNoDTDStillPasses(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?><root><key>value</key></root>`
	valid, err := XMLValidator{}.ValidateSyntax([]byte(xml))
	require.True(t, valid)
	require.NoError(t, err)
}

func writeTestXSD(t *testing.T) string {
	t.Helper()
	xsd := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<xs:schema xmlns:xs="` + xsNamespace + `">` + // DevSkim: ignore DS137138
		`<xs:element name="config"><xs:complexType><xs:sequence>` +
		`<xs:element name="host" type="xs:string"/>` +
		`<xs:element name="port" type="xs:integer"/>` +
		`</xs:sequence></xs:complexType></xs:element></xs:schema>`
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.xsd")
	require.NoError(t, os.WriteFile(p, []byte(xsd), 0600))
	return p
}

func Test_JSONCValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	valid, err := JSONCValidator{}.ValidateSchema([]byte(`// comment
{"key": "value"}`), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_JSONCValidateSchemaValid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	doc := `// server config
{
  "$schema": "` + schema + `",
  "host": "db.example.com",
  "port": 5432,
  "database": "mydb",
}`
	valid, err := JSONCValidator{}.ValidateSchema([]byte(doc), "")
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_JSONCValidateSchemaInvalidDoc(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	doc := `// server config
{
  "$schema": "` + schema + `",
  "host": "db.example.com",
  "port": "not_a_number", // wrong type
  "database": "mydb"
}`
	valid, err := JSONCValidator{}.ValidateSchema([]byte(doc), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_JSONCMarshalToJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`// comment
{
  "$schema": "test.json",
  "key": "value",
}`)
	out, err := JSONCValidator{}.MarshalToJSON(input)
	require.NoError(t, err)
	require.NotContains(t, string(out), "$schema")
	require.Contains(t, string(out), "key")
}

func Test_JSONCValidateSchemaEmptySchema(t *testing.T) {
	t.Parallel()
	valid, err := JSONCValidator{}.ValidateSchema([]byte(`{"$schema": "", "key": "value"}`), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must not be empty")
}

func Test_JSONCValidateSchemaArrayRoot(t *testing.T) {
	t.Parallel()
	valid, err := JSONCValidator{}.ValidateSchema([]byte(`[1, 2, 3]`), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_JSONCValidateSchemaInvalidSyntax(t *testing.T) {
	t.Parallel()
	valid, err := JSONCValidator{}.ValidateSchema([]byte(`{bad`), "")
	require.False(t, valid)
	require.Error(t, err)
}

func Test_JSONCValidateSchemaNonStringSchema(t *testing.T) {
	t.Parallel()
	valid, err := JSONCValidator{}.ValidateSchema([]byte(`{"$schema": 123}`), "")
	require.False(t, valid)
	require.ErrorContains(t, err, "$schema must be a string")
}

func Test_JSONCMarshalToJSONArrayRoot(t *testing.T) {
	t.Parallel()
	out, err := JSONCValidator{}.MarshalToJSON([]byte(`// comment
[1, 2, 3]`))
	require.NoError(t, err)
	require.Contains(t, string(out), "1")
}

func Test_JSONCMarshalToJSONInvalid(t *testing.T) {
	t.Parallel()
	_, err := JSONCValidator{}.MarshalToJSON([]byte(`{bad`))
	require.Error(t, err)
}

func Test_CsvValidatorSemicolonDelimiter(t *testing.T) {
	t.Parallel()
	v := CsvValidator{Delimiter: ';'}
	valid, err := v.ValidateSyntax([]byte("name;age\nAlice;30\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_CsvValidatorSemicolonDelimiterInvalid(t *testing.T) {
	t.Parallel()
	v := CsvValidator{Delimiter: ';'}
	valid, err := v.ValidateSyntax([]byte("name;age;city\nAlice;30\n"))
	require.False(t, valid)
	require.Error(t, err)
}

func Test_CsvValidatorTabDelimiter(t *testing.T) {
	t.Parallel()
	v := CsvValidator{Delimiter: '\t'}
	valid, err := v.ValidateSyntax([]byte("name\tage\nAlice\t30\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_CsvValidatorComment(t *testing.T) {
	t.Parallel()
	v := CsvValidator{Comment: '#'}
	valid, err := v.ValidateSyntax([]byte("# comment\nname,age\nAlice,30\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_CsvValidatorLazyQuotes(t *testing.T) {
	t.Parallel()
	input := []byte("name,desc\nAlice,she said \"hello\"\n")
	// Strict mode fails
	v1 := CsvValidator{}
	valid, _ := v1.ValidateSyntax(input)
	require.False(t, valid)
	// Lazy quotes mode passes
	v2 := CsvValidator{LazyQuotes: true}
	valid, err := v2.ValidateSyntax(input)
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_CsvValidatorDefaultUnchanged(t *testing.T) {
	t.Parallel()
	v := CsvValidator{}
	valid, err := v.ValidateSyntax([]byte("a,b,c\n1,2,3\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_JSONDuplicateKeysAllowed(t *testing.T) {
	t.Parallel()
	v := JSONValidator{}
	valid, err := v.ValidateSyntax([]byte(`{"key": 1, "key": 2}`))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_JSONDuplicateKeysForbidden(t *testing.T) {
	t.Parallel()
	v := JSONValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte(`{"key": 1, "key": 2}`))
	require.False(t, valid)
	require.ErrorContains(t, err, `duplicate key "key"`)
}

func Test_JSONDuplicateKeysNested(t *testing.T) {
	t.Parallel()
	v := JSONValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte(`{"outer": {"inner": 1, "inner": 2}}`))
	require.False(t, valid)
	require.ErrorContains(t, err, `duplicate key "inner"`)
}

func Test_JSONDuplicateKeysInArray(t *testing.T) {
	t.Parallel()
	v := JSONValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte(`[{"a": 1}, {"a": 1}]`))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_JSONNoDuplicatesPasses(t *testing.T) {
	t.Parallel()
	v := JSONValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte(`{"a": 1, "b": 2, "c": 3}`))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_INIDuplicateKeysAllowed(t *testing.T) {
	t.Parallel()
	v := IniValidator{}
	valid, err := v.ValidateSyntax([]byte("[section]\nkey=1\nkey=2\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_INIDuplicateKeysForbidden(t *testing.T) {
	t.Parallel()
	v := IniValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte("[section]\nkey=1\nkey=2\n"))
	require.False(t, valid)
	require.ErrorContains(t, err, `duplicate key "key"`)
}

func Test_ININoDuplicatesPasses(t *testing.T) {
	t.Parallel()
	v := IniValidator{ForbidDuplicateKeys: true}
	valid, err := v.ValidateSyntax([]byte("[section]\na=1\nb=2\n"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_buildJSONPositionMap(t *testing.T) {
	t.Parallel()
	doc := []byte("{\n  \"name\": \"app\",\n  \"port\": 8080,\n  \"nested\": {\n    \"key\": \"val\"\n  }\n}")
	positions := buildJSONPositionMap(doc)

	require.Contains(t, positions, "(root)")
	require.Contains(t, positions, "(root).name")
	require.Contains(t, positions, "(root).port")
	require.Contains(t, positions, "(root).nested")
	require.Contains(t, positions, "(root).nested.key")

	require.Equal(t, 2, positions["(root).name"].Line)
	require.Equal(t, 4, positions["(root).nested"].Line)
	require.Equal(t, 5, positions["(root).nested.key"].Line)
}

func Test_buildJSONPositionMapArray(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"items": [1, 2, 3]}`)
	positions := buildJSONPositionMap(doc)
	require.Contains(t, positions, "(root).items")
}

func Test_buildJSONPositionMapEmpty(t *testing.T) {
	t.Parallel()
	positions := buildJSONPositionMap([]byte(`{}`))
	require.Contains(t, positions, "(root)")
}

func Test_buildYAMLPositionMap(t *testing.T) {
	t.Parallel()
	doc := []byte("name: app\nserver:\n  host: example.local\n  port: 8080\n")
	positions := buildYAMLPositionMap(doc)

	require.Contains(t, positions, "(root)")
	require.Contains(t, positions, "(root).name")
	require.Contains(t, positions, "(root).server")
	require.Contains(t, positions, "(root).server.host")
	require.Contains(t, positions, "(root).server.port")

	require.Equal(t, 1, positions["(root).name"].Line)
	require.Equal(t, 3, positions["(root).server.host"].Line)
	require.Equal(t, 4, positions["(root).server.port"].Line)
}

func Test_buildYAMLPositionMapSequence(t *testing.T) {
	t.Parallel()
	doc := []byte("items:\n  - one\n  - two\n  - three\n")
	positions := buildYAMLPositionMap(doc)

	require.Contains(t, positions, "(root).items")
	require.Contains(t, positions, "(root).items.0")
	require.Contains(t, positions, "(root).items.1")
	require.Contains(t, positions, "(root).items.2")
	require.Equal(t, 2, positions["(root).items.0"].Line)
	require.Equal(t, 4, positions["(root).items.2"].Line)
}

func Test_buildYAMLPositionMapInvalidYAML(t *testing.T) {
	t.Parallel()
	positions := buildYAMLPositionMap([]byte("a: b\nc: d:::::::::::::::"))
	require.Nil(t, positions)
}

func Test_JSONSchemaErrorPositions(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	doc := []byte("{\n  \"$schema\": \"" + schema + "\",\n  \"host\": \"db.example.com\",\n  \"port\": \"not_a_number\",\n  \"database\": \"mydb\"\n}")
	valid, err := JSONValidator{}.ValidateSchema(doc, "")
	require.False(t, valid)

	var se *SchemaErrors
	require.ErrorAs(t, err, &se)
	require.NotEmpty(t, se.Positions)
	// port error should have a line > 0
	for i, item := range se.Items {
		if item == "port: Invalid type. Expected: integer, given: string" {
			require.Positive(t, se.Positions[i].Line)
		}
	}
}

func Test_YAMLSchemaErrorPositions(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "# yaml-language-server: $schema=" + schema + "\nhost: db.example.com\nport: not_a_number\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.False(t, valid)

	var se *SchemaErrors
	require.ErrorAs(t, err, &se)
	require.NotEmpty(t, se.Positions)
	for i, item := range se.Items {
		if item == "port: Invalid type. Expected: integer, given: string" {
			require.Equal(t, 3, se.Positions[i].Line)
		}
	}
}

func Test_SchemaErrorPositionZeroWhenNoMatch(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	// Missing required field "host" — no position since the key doesn't exist
	doc := []byte("{\n  \"$schema\": \"" + schema + "\",\n  \"port\": 5432,\n  \"database\": \"mydb\"\n}")
	valid, err := JSONValidator{}.ValidateSchema(doc, "")
	require.False(t, valid)

	var se *SchemaErrors
	require.ErrorAs(t, err, &se)
	// "host is required" error is on (root) which should have a position
	// but the missing field itself has no position — (root) maps to line 1
	require.NotEmpty(t, se.Positions)
}

func Test_JustfileValidateSyntaxError(t *testing.T) {
	t.Parallel()
	valid, err := JustfileValidator{}.ValidateSyntax([]byte("name := \"unterminated\n"))
	require.False(t, valid)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Positive(t, ve.Line)
}

func Test_JustfileValidateSemanticError(t *testing.T) {
	t.Parallel()
	// Recipe depends on undefined recipe
	valid, err := JustfileValidator{}.ValidateSyntax([]byte("build: nonexistent\n    echo build\n"))
	require.False(t, valid)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Contains(t, ve.Err.Error(), "undefined recipe")
}

func Test_JustfileValidateValid(t *testing.T) {
	t.Parallel()
	valid, err := JustfileValidator{}.ValidateSyntax([]byte("# comment\ndefault:\n    echo hello\n"))
	require.True(t, valid)
	require.NoError(t, err)
}
