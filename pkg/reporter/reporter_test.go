package reporter

import (
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
	}

	backslashReport = Report{
		"good.xml",
		"\\fake\\path\\good.xml",
		true,
		nil,
		false,
	}

	invalidReport = Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
		false,
	}

	multiLineErrorReport = Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
		false,
	}

	quietReport = Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
		true,
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
		"test/output/example/good.json",
		true,
		nil,
		false,
	}

	type testCase struct {
		name       string
		reporter   Reporter
		outputDest string
		goldenFile string
		wantErr    assert.ErrorAssertionFunc
	}

	for _, tc := range []struct {
		name       string
		newReporter func(string) Reporter
		extension  string
		goldenFile string
		jsonEq     bool
	}{
		{"json", func(d string) Reporter { return NewJSONReporter(d) }, "json", "../../test/output/example/result.json", true},
		{"junit", func(d string) Reporter { return NewJunitReporter(d) }, "xml", "../../test/output/example/result.xml", false},
		{"sarif", func(d string) Reporter { return NewSARIFReporter(d) }, "sarif", "../../test/output/example/result.sarif", false},
	} {
		t.Run(tc.name+" to dir", func(t *testing.T) {
			tmpDir := t.TempDir()
			err := tc.newReporter(tmpDir).Print([]Report{report})
			require.NoError(t, err)

			golden, err := os.ReadFile(tc.goldenFile)
			require.NoError(t, err)
			actual, err := os.ReadFile(tmpDir + "/result." + tc.extension)
			require.NoError(t, err)

			if tc.jsonEq {
				assert.JSONEq(t, string(golden), string(actual))
			} else {
				assert.Equal(t, golden, actual)
			}
		})

		t.Run(tc.name+" to file", func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := tmpDir + "/validator_result." + tc.extension
			err := tc.newReporter(outPath).Print([]Report{report})
			require.NoError(t, err)

			golden, err := os.ReadFile(tc.goldenFile)
			require.NoError(t, err)
			actual, err := os.ReadFile(outPath)
			require.NoError(t, err)

			if tc.jsonEq {
				assert.JSONEq(t, string(golden), string(actual))
			} else {
				assert.Equal(t, golden, actual)
			}
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

// --- Helpers ---

func assertRegexpError(regexp any) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, got error, msg ...any) bool {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		//nolint:testifylint // in this use case it's ok to use assert.Error
		return assert.Error(t, got, msg...) && assert.Regexp(t, regexp, got.Error(), msg...)
	}
}


