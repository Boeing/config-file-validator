package validator

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	{"validSarif210", validSarif210Bytes, true, SarifValidator{}},
	{"validSarif22", validSarif22Bytes, true, SarifValidator{}},
	{"invalidSarif", []byte(`{"not": "sarif"}`), false, SarifValidator{}},
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

func Test_getCustomErrNonSyntaxError(t *testing.T) {
	t.Parallel()
	// Unmarshal into a typed struct to trigger UnmarshalTypeError instead of SyntaxError
	var target struct {
		Key int `json:"key"`
	}
	input := []byte(`{"key": "not_a_number"}`)
	err := json.Unmarshal(input, &target)
	if err == nil {
		t.Fatal("expected an error")
	}
	customErr := getCustomErr(input, err)
	// Should return the original error unchanged since it's not a SyntaxError
	if !errors.Is(customErr, err) {
		t.Errorf("expected original error, got: %v", customErr)
	}
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
	// JSON with empty $schema should return ErrNoSchema
	valid, err := JSONValidator{}.ValidateSchema([]byte(`{"$schema": "", "key": "value"}`), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
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

func Test_YAMLValidateSchemaNoSchema(t *testing.T) {
	t.Parallel()
	valid, err := YAMLValidator{}.ValidateSchema([]byte("key: value\n"), "")
	require.True(t, valid)
	require.ErrorIs(t, err, ErrNoSchema)
}

func Test_YAMLValidateSchemaWithComment(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "# yaml-language-server: $schema=" + schema + "\nhost: localhost\nport: 5432\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_YAMLValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "# yaml-language-server: $schema=" + schema + "\nhost: localhost\nport: not_a_number\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.False(t, valid)
	require.ErrorContains(t, err, "schema validation failed")
}

func Test_YAMLValidateSchemaCommentAfterBlank(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	yaml := "\n# yaml-language-server: $schema=" + schema + "\nhost: localhost\nport: 5432\ndatabase: mydb\n"
	valid, err := YAMLValidator{}.ValidateSchema([]byte(yaml), filepath.Join(filepath.Dir(schema), "test.yaml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_YAMLValidateSchemaCommentAfterContent(t *testing.T) {
	t.Parallel()
	// Schema comment after non-comment content should be ignored
	yaml := "key: value\n# yaml-language-server: $schema=http://example.com/schema.json\n"
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
	toml := `"$schema" = "` + schema + "\"\nhost = \"localhost\"\nport = 5432\ndatabase = \"mydb\"\n"
	valid, err := TomlValidator{}.ValidateSchema([]byte(toml), filepath.Join(filepath.Dir(schema), "test.toml"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_TomlValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toml := `"$schema" = "` + schema + "\"\nhost = \"localhost\"\nport = \"not_a_number\"\ndatabase = \"mydb\"\n"
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
	toonDoc := "\"$schema\": " + schema + "\nhost: localhost\nport: 5432\ndatabase: mydb\n"
	valid, err := ToonValidator{}.ValidateSchema([]byte(toonDoc), filepath.Join(filepath.Dir(schema), "test.toon"))
	require.True(t, valid)
	require.NoError(t, err)
}

func Test_ToonValidateSchemaInvalid(t *testing.T) {
	t.Parallel()
	schema := writeTestSchema(t)
	toonDoc := "\"$schema\": " + schema + "\nhost: localhost\nport: \"not_a_number\"\ndatabase: mydb\n"
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
		{"standard", "# yaml-language-server: $schema=http://example.com/s.json\nkey: val", "http://example.com/s.json"},
		{"with spaces", "#  yaml-language-server:  $schema=http://example.com/s.json \nkey: val", "http://example.com/s.json"},
		{"blank lines before", "\n\n# yaml-language-server: $schema=http://example.com/s.json\nkey: val", "http://example.com/s.json"},
		{"no comment", "key: val", ""},
		{"wrong comment", "# just a comment\nkey: val", ""},
		{"after content", "key: val\n# yaml-language-server: $schema=http://example.com/s.json", ""},
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
