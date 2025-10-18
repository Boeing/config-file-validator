package formatter

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var testData = []struct {
	name           string
	testInput      []byte
	expectedResult []byte
	expectedError  bool
	formatter      Formatter
}{
	{"validJSON", []byte(`{"test": "test"}`), []byte("{\n  \"test\": \"test\"\n}"), false, JSONFormatter{}},
	{"unformattedJSON", []byte(`{"test": "test"}`), []byte("{\n  \"test\": \"test\"\n}"), false, JSONFormatter{}},
	{"emptyJSON", []byte("{}"), []byte("{}"), false, JSONFormatter{}},
	{"jsonArray", []byte(`[1,2,3]`), []byte("[\n  1,\n  2,\n  3\n]"), false, JSONFormatter{}},
	{"nestedJSON", []byte(`{"a":{"b":{"c":"d"}}}`), []byte("{\n  \"a\": {\n    \"b\": {\n      \"c\": \"d\"\n    }\n  }\n}"), false, JSONFormatter{}},
	{"alreadyFormattedJSON", []byte("{\n  \"test\": \"value\"\n}\n"), []byte("{\n  \"test\": \"value\"\n}\n"), false, JSONFormatter{}},
	{"emptyInput", []byte(``), nil, true, JSONFormatter{}},
}

func Test_ValidationInput(t *testing.T) {
	t.Parallel()

	for _, tcase := range testData {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			// create temp file with test input
			tmpFile, err := os.CreateTemp("", "test-*.json")
			require.NoError(t, err, "failed to create temp file")
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.Write(tcase.testInput)
			require.NoError(t, err, "failed to write to temp file")
			tmpFile.Close()

			err = tcase.formatter.Format(tmpFile.Name())
			if tcase.expectedError {
				require.Error(t, err, "expected error but got nil")
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
				fileContent, err := os.ReadFile(tmpFile.Name())
				require.NoError(t, err, "failed to read formatted file")
				require.True(t,
					bytes.Equal(fileContent, tcase.expectedResult),
					"expected result: %s, got: %s", tcase.expectedResult, fileContent)
			}
		})
	}
}
