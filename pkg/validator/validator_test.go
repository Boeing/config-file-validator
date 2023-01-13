package validator

import (
	"testing"
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
}

func Test_ValidationInput(t *testing.T) {
	for _, d := range testData {
		valid, err := d.validator.Validate(d.testInput)
		if valid != d.expectedResult {
			t.Errorf("incorrect result: expected %v, got %v", d.expectedResult, valid)
		}

		if valid && err != nil {
			t.Error("incorrect result: err was not nil", err)
		}

		if !valid && err == nil {
			t.Error("incorrect result: function returned a nil error")
		}
	}
}
