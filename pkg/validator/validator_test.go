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
	{"validMakefile", []byte("all:\n\t@echo hi\n"), true, MakefileValidator{}},
	{"invalidMakefile_no_recipe", []byte("all:\n"), false, MakefileValidator{}},
	{"invalidMakefile_spaces_instead_of_tabs", []byte("all:\n    echo bad\n"), false, MakefileValidator{}},
	{"invalidMakefile_recipe_before_target", []byte("\t@echo bad\nall:\n\t@echo hi\n"), false, MakefileValidator{}},
	{"validJustfile", []byte("build:\n\techo hi\n"), true, JustfileValidator{}},
	{"validJustfile_spaces", []byte("build:\n  echo hi\n"), true, JustfileValidator{}},
	{"invalidJustfile_no_rule", []byte("echo hi\n"), false, JustfileValidator{}},
	{"invalidJustfile_no_commands", []byte("build:\n\ntest:\n  echo hi\n"), false, JustfileValidator{}},
	{"invalidJustfile_command_before_rule", []byte("  echo bad\nbuild:\n  echo hi\n"), false, JustfileValidator{}},
}

func Test_ValidationInput(t *testing.T) {
	t.Parallel()

	for _, tcase := range testData {
		tcase := tcase // Capture the range variable

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
