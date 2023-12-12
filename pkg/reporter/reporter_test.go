package reporter

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_stdoutReport(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReport(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithBackslashPath := Report{
		"good.xml",
		"\\fake\\path\\good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithBackslashPath}

	jsonReporter := JsonReporter{}
	err := jsonReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_junitReport(t *testing.T) {
	prop1 := Property{Name: "property1", Value: "value", TextValue: "text value"}
	properties := []Property{prop1}
	testsuite := Testsuite{Name: "config-file-validator", Errors: 0, Properties: &properties}
	testsuiteBatch := []Testsuite{testsuite}
	ts := Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err := ts.getReport()
	if err == nil {
		t.Errorf("Reporting failed on getReport")
	}

	prop2 := Property{Name: "property2", Value: "value"}
	properties2 := []Property{prop2}
	testsuite = Testsuite{Name: "config-file-validator", Errors: 0, Properties: &properties2}
	testsuiteBatch = []Testsuite{testsuite}
	ts = Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err = ts.getReport()
	if err != nil {
		t.Errorf("Reporting failed on getReport")
	}

	tc1 := Testcase{Name: "testcase2", ClassName: "config-file-validator", Properties: &properties}
	testCasesBatch := []Testcase{tc1}
	testsuite = Testsuite{Name: "config-file-validator", Errors: 0, Testcases: &testCasesBatch}
	testsuiteBatch = []Testsuite{testsuite}
	ts3 := Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err = ts3.getReport()
	if err == nil {
		t.Errorf("Reporting failed on getReport")
	}

	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithBackslashPath := Report{
		"good.xml",
		"\\fake\\path\\good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reports := []Report{reportNoValidationError, reportWithBackslashPath, reportWithValidationError}

	junitReporter := JunitReporter{}
	err = junitReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReporterWriter(t *testing.T) {
	var (
		report = Report{
			"good.json",
			"test/output/example/good.json",
			true,
			nil,
		}
	)
	deleteFiles(t)

	bytes, err := os.ReadFile("../../test/output/example/result.json")
	require.NoError(t, err)

	type args struct {
		reports    []Report
		outputDest string
	}
	type want struct {
		data []byte
		err  assert.ErrorAssertionFunc
	}

	tests := map[string]struct {
		args args
		want want
	}{
		"Normal/Output results to a file named 'result.json' (default name)": {
			args: args{
				reports: []Report{
					report,
				},
				outputDest: "../../test/output",
			},
			want: want{
				data: bytes,
				err:  assert.NoError,
			},
		},
		"Normal/Output results to a file with a given name": {
			args: args{
				reports: []Report{
					report,
				},
				outputDest: "../../test/output/validator_result.json",
			},
			want: want{
				data: bytes,
				err:  assert.NoError,
			},
		},
		"Abnormal/a non-existing dir for output is specified": {
			args: args{
				reports: []Report{
					report,
				},
				outputDest: "../../test/wrong/output",
			},
			want: want{
				data: nil,
				err:  assertRegexpError("failed to create a file: "),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sut := NewJsonReporter(tt.args.outputDest)
			err := sut.Print(tt.args.reports)
			tt.want.err(t, err)
			if tt.want.data != nil {
				fileName := ""
				if info, _ := os.Stat(tt.args.outputDest); info.IsDir() {
					fileName = "/result.json"
				}
				bytes, err := os.ReadFile(tt.args.outputDest + fileName)
				require.NoError(t, err)
				assert.Equal(t, tt.want.data, bytes)
				err = os.Remove(tt.args.outputDest + fileName)
				require.NoError(t, err)
			}
		},
		)
	}
}

func assertErrorIs(expectation error) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, got error, msg ...interface{}) bool {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		return assert.ErrorIs(t, got, expectation, msg...)
	}
}

func assertRegexpError(regexp interface{}) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, got error, msg ...interface{}) bool {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		return assert.Error(t, got, msg...) && assert.Regexp(t, regexp, got.Error(), msg...)
	}
}

// deleteFiles deletes all files for tests under the 'test/output' directory, except those under 'test/output/example'.
func deleteFiles(t *testing.T) {
	t.Helper()
	directoryPath := "../../test/output"

	files, err := filepath.Glob(filepath.Join(directoryPath, "*"))
	require.NoError(t, err)

	var filteredFiles []string
	for _, file := range files {
		_, dirName := filepath.Split(file)
		if dirName != "example" {
			filteredFiles = append(filteredFiles, file)
		}
	}

	for _, file := range filteredFiles {
		err := os.Remove(file)
		require.NoError(t, err)
	}
}
