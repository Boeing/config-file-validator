package validator

import (
	_ "embed"
	"testing"
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
}

func Test_ValidationInput(t *testing.T) {
	t.Parallel()

	for _, tcase := range testData {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			valid, err := tcase.validator.Validate(tcase.testInput)
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
		_, _ = v.Validate(a)
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
