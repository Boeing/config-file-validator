package formatter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var validJson = []byte(`{"test": "test"}`)
var formattedJson = []byte(`{
  "test": "test"
}`)

var testData = []struct {
	name           string
	testInput      []byte
	expectedResult []byte
	expectedError  bool
	formatter      Formatter
}{
	{"validJson", validJson, formattedJson, false, JSONFormatter{}},
	{"invalidJson", []byte(`{test": "test"}`), nil, true, JSONFormatter{}},
	{"unformattedJson", validJson, formattedJson, false, JSONFormatter{}},
	{"emptyJson", []byte(`{}`), []byte(`{}`), false, JSONFormatter{}},
	{"jsonArray", []byte(`[1,2,3]`), []byte("[\n  1,\n  2,\n  3\n]"), false, JSONFormatter{}},
	{"nestedJson", []byte(`{"a":{"b":{"c":"d"}}}`), []byte("{\n  \"a\": {\n    \"b\": {\n      \"c\": \"d\"\n    }\n  }\n}"), false, JSONFormatter{}},
	{"alreadyFormattedJson", []byte("{\n  \"test\": \"value\"\n}"), []byte("{\n  \"test\": \"value\"\n}"), false, JSONFormatter{}},
	{"jsonWithComments", []byte(`{"valid":"json"}// comments not allowed`), nil, true, JSONFormatter{}},
	{"incompleteJson", []byte(`{"incomplete"`), nil, true, JSONFormatter{}},
	{"emptyInput", []byte(``), nil, true, JSONFormatter{}},
}

func Test_ValidationInput(t *testing.T) {
	t.Parallel()

	for _, tcase := range testData {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			formatted, err := tcase.formatter.Format(tcase.testInput)
			if tcase.expectedError {
				assert.Error(t, err, "expected error but got nil")
			} else {
				assert.NoError(t, err, "expected no error but got: %v", err)
				assert.True(t,
					bytes.Equal(formatted, tcase.expectedResult),
					"expected result: %s, got: %s", tcase.expectedResult, formatted)
			}
		})
	}
}
