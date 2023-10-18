package validator

import (
	_ "embed"
	"testing"
)

var (
	//go:embed valid.plist
	validPlistBytes []byte

	//go:embed invalid.plist
	invalidPlistBytes []byte
)

var testData = []struct {
	name           string
	testInput      []byte
	expectedResult bool
	validator      Validator
}{
	{"validJson", []byte(`{"test": "test"}`), true, JsonValidator{}},
	{"invalidJson", []byte(`{test": "test"}`), false, JsonValidator{}},
	{"validYaml", []byte("a: 1\nb: 2"), true, YamlValidator{}},
	{"invalidYaml", []byte("a: b\nc: d:::::::::::::::"), false, YamlValidator{}},
	{"validXml", []byte("<test>\n</test>"), true, XmlValidator{}},
	{"invalidXml", []byte("<xml\n"), false, XmlValidator{}},
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
