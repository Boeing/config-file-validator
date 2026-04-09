package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared test fixtures
var (
	validReport = Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
		false,
		0,
		0,
	}

	backslashReport = Report{
		"good.xml",
		"\\fake\\path\\good.xml",
		true,
		nil,
		false,
		0,
		0,
	}

	invalidReport = Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("unable to parse bad.xml file"),
		false,
		0,
		0,
	}

	multiLineErrorReport = Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("unable to parse keys:\nkey1\nkey2"),
		false,
		0,
		0,
	}

	quietReport = Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
		true,
		0,
		0,
	}

	mixedReports = []Report{validReport, invalidReport, multiLineErrorReport}
)

// --- Basic Print tests ---

func Test_stdoutReport(t *testing.T) {
	err := NewStdoutReporter("").Print(mixedReports)
	require.NoError(t, err)
}

func Test_stdoutReportQuiet(t *testing.T) {
	err := NewStdoutReporter("").Print([]Report{quietReport})
	require.NoError(t, err)
}

func Test_stdoutReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := NewStdoutReporter(tmpDir).Print([]Report{validReport})
	require.NoError(t, err)
}

func Test_jsonReport(t *testing.T) {
	reports := []Report{validReport, invalidReport, backslashReport}
	err := (&JSONReporter{}).Print(reports)
	require.NoError(t, err)
}

func Test_jsonReportQuiet(t *testing.T) {
	err := (&JSONReporter{}).Print([]Report{quietReport})
	require.NoError(t, err)
}

func Test_jsonReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := NewJSONReporter(tmpDir).Print([]Report{validReport})
	require.NoError(t, err)
}

func Test_junitReport(t *testing.T) {
	reports := []Report{validReport, backslashReport, {
		"bad.xml",
		"/fake/path/bad.json",
		false,
		errors.New("Incorrect characters '<' and '</>` found in file"),
		false,
		0,
		0,
	}}
	err := (JunitReporter{}).Print(reports)
	require.NoError(t, err)
}

func Test_junitGetReport(t *testing.T) {
	// Property with TextValue should fail
	prop1 := Property{Name: "property1", Value: "value", TextValue: "text value"}
	ts := Testsuites{Name: "cfv", Tests: 1, Testsuites: []Testsuite{
		{Name: "cfv", Errors: 0, Properties: &[]Property{prop1}},
	}}
	_, err := ts.getReport()
	require.Error(t, err)

	// Property without TextValue should succeed
	prop2 := Property{Name: "property2", Value: "value"}
	ts2 := Testsuites{Name: "cfv", Tests: 1, Testsuites: []Testsuite{
		{Name: "cfv", Errors: 0, Properties: &[]Property{prop2}},
	}}
	_, err = ts2.getReport()
	require.NoError(t, err)

	// Testcase with bad property should fail
	tc := Testcase{Name: "tc", ClassName: "cfv", Properties: &[]Property{prop1}}
	ts3 := Testsuites{Name: "cfv", Tests: 1, Testsuites: []Testsuite{
		{Name: "cfv", Errors: 0, Testcases: &[]Testcase{tc}},
	}}
	_, err = ts3.getReport()
	require.Error(t, err)
}

func Test_sarifReport(t *testing.T) {
	reports := []Report{validReport, invalidReport, backslashReport}
	err := (&SARIFReporter{}).Print(reports)
	require.NoError(t, err)
}

func Test_sarifReportWithRegion(t *testing.T) {
	reportWithPos := Report{
		"bad.json",
		"/fake/path/bad.json",
		false,
		errors.New("error at line 3 column 10"),
		false,
		3,
		10,
	}
	reportLineOnly := Report{
		"bad.yaml",
		"/fake/path/bad.yaml",
		false,
		errors.New("yaml: line 5: mapping error"),
		false,
		5,
		0,
	}

	var buf bytes.Buffer
	log, err := createSARIFReport([]Report{reportWithPos, reportLineOnly, validReport})
	require.NoError(t, err)

	sarifBytes, err := json.MarshalIndent(log, "", "  ")
	require.NoError(t, err)
	buf.Write(sarifBytes)

	output := buf.String()
	// reportWithPos should have region with startLine and startColumn
	assert.Contains(t, output, `"startLine": 3`)
	assert.Contains(t, output, `"startColumn": 10`)
	// reportLineOnly should have region with startLine only (no startColumn since it's 0)
	assert.Contains(t, output, `"startLine": 5`)
	// validReport should not have a region
	assert.NotContains(t, output, `"startLine": 0`)
}

func Test_sarifReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := NewSARIFReporter(tmpDir).Print([]Report{validReport})
	require.NoError(t, err)
}

// --- Grouped stdout tests ---

func Test_stdoutGroupedReports(t *testing.T) {
	singleGroup := map[string][]Report{
		"xml": mixedReports,
	}
	err := PrintSingleGroupStdout(singleGroup)
	require.NoError(t, err)

	// With "Passed"/"Failed" keys to hit checkGroupsForPassFail returning false
	passfailGroup := map[string][]Report{
		"Passed": {validReport},
		"Failed": {invalidReport},
	}
	err = PrintSingleGroupStdout(passfailGroup)
	require.NoError(t, err)

	doubleGroup := map[string]map[string][]Report{
		"xml": {"directory": mixedReports},
	}
	err = PrintDoubleGroupStdout(doubleGroup)
	require.NoError(t, err)

	tripleGroup := map[string]map[string]map[string][]Report{
		"xml": {"directory": {"pass-fail": mixedReports}},
	}
	err = PrintTripleGroupStdout(tripleGroup)
	require.NoError(t, err)
}

// --- Grouped JSON tests ---

func Test_jsonGroupedReports(t *testing.T) {
	singleGroup := map[string][]Report{
		"xml": mixedReports,
	}
	err := PrintSingleGroupJSON(singleGroup)
	require.NoError(t, err)

	doubleGroup := map[string]map[string][]Report{
		"xml": {"directory": mixedReports},
	}
	err = PrintDoubleGroupJSON(doubleGroup)
	require.NoError(t, err)

	tripleGroup := map[string]map[string]map[string][]Report{
		"xml": {"directory": {"pass-fail": mixedReports}},
	}
	err = PrintTripleGroupJSON(tripleGroup)
	require.NoError(t, err)
}

// --- Reporter file output tests (shared pattern) ---

func Test_reporterFileOutput(t *testing.T) {
	report := Report{
		"good.json",
		"/fake/path/good.json",
		true,
		nil,
		false,
		0,
		0,
	}

	for _, tc := range []struct {
		name        string
		newReporter func(string) Reporter
		extension   string
		verify      func(t *testing.T, data []byte)
	}{
		{
			"json",
			func(d string) Reporter { return NewJSONReporter(d) },
			"json",
			func(t *testing.T, data []byte) {
				t.Helper()
				assert.Contains(t, string(data), `"status": "passed"`)
				assert.Contains(t, string(data), `"passed": 1`)
				assert.Contains(t, string(data), `"/fake/path/good.json"`)
			},
		},
		{
			"junit",
			func(d string) Reporter { return NewJunitReporter(d) },
			"xml",
			func(t *testing.T, data []byte) {
				t.Helper()
				assert.Contains(t, string(data), `<?xml version="1.0" encoding="UTF-8"?>`)
				assert.Contains(t, string(data), `config-file-validator`)
				assert.Contains(t, string(data), `/fake/path/good.json`)
			},
		},
		{
			"sarif",
			func(d string) Reporter { return NewSARIFReporter(d) },
			"sarif",
			func(t *testing.T, data []byte) {
				t.Helper()
				assert.Contains(t, string(data), `"version": "2.1.0"`)
				assert.Contains(t, string(data), `"kind": "pass"`)
				assert.Contains(t, string(data), `/fake/path/good.json`)
			},
		},
	} {
		t.Run(tc.name+" to dir", func(t *testing.T) {
			tmpDir := t.TempDir()
			err := tc.newReporter(tmpDir).Print([]Report{report})
			require.NoError(t, err)

			actual, err := os.ReadFile(tmpDir + "/result." + tc.extension)
			require.NoError(t, err)
			tc.verify(t, actual)
		})

		t.Run(tc.name+" to file", func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := tmpDir + "/validator_result." + tc.extension
			err := tc.newReporter(outPath).Print([]Report{report})
			require.NoError(t, err)

			actual, err := os.ReadFile(outPath)
			require.NoError(t, err)
			tc.verify(t, actual)
		})

		t.Run(tc.name+" to stdout", func(t *testing.T) {
			err := tc.newReporter("").Print([]Report{report})
			require.NoError(t, err)
		})

		t.Run(tc.name+" to bad path", func(t *testing.T) {
			err := tc.newReporter("/nonexistent/path/output").Print([]Report{report})
			require.Error(t, err)
			assert.Regexp(t, "failed to create a file", err.Error())
		})
	}
}

// --- checkGroupsForPassFail ---

func Test_checkGroupsForPassFail(t *testing.T) {
	require.True(t, checkGroupsForPassFail("xml", "directory"))
	require.False(t, checkGroupsForPassFail("Passed"))
	require.False(t, checkGroupsForPassFail("xml", "Failed"))
}
