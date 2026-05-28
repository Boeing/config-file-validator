package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared test fixtures
var (
	validReport = Report{
		FileName: "good.xml",
		FilePath: "/fake/path/good.xml",
		IsValid:  true,
	}

	backslashReport = Report{
		FileName: "good.xml",
		FilePath: "\\fake\\path\\good.xml",
		IsValid:  true,
	}

	invalidReport = Report{
		FileName:         "bad.xml",
		FilePath:         "/fake/path/bad.xml",
		IsValid:          false,
		ValidationError:  errors.New("unable to parse bad.xml file"),
		ValidationErrors: []string{"unable to parse bad.xml file"},
	}

	multiLineErrorReport = Report{
		FileName:         "bad.xml",
		FilePath:         "/fake/path/bad.xml",
		IsValid:          false,
		ValidationError:  errors.New("unable to parse keys:\nkey1\nkey2"),
		ValidationErrors: []string{"unable to parse keys:\nkey1\nkey2"},
	}

	quietReport = Report{
		FileName: "good.xml",
		FilePath: "/fake/path/good.xml",
		IsValid:  true,
		IsQuiet:  true,
	}

	mixedReports = []Report{validReport, invalidReport, multiLineErrorReport}
)

func captureStdout(t *testing.T, writeOutput func() error) (string, error) {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	printErr := writeOutput()
	os.Stdout = originalStdout

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	return string(output), printErr
}

func decodeJSONOutput(t *testing.T, output string) map[string]any {
	t.Helper()

	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &report))
	return report
}

func requireJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()

	actual, ok := value.(map[string]any)
	require.Truef(t, ok, "expected JSON object, got %T", value)
	return actual
}

func requireJSONArray(t *testing.T, value any) []any {
	t.Helper()

	actual, ok := value.([]any)
	require.Truef(t, ok, "expected JSON array, got %T", value)
	return actual
}

func requireJSONNumber(t *testing.T, value any, expected float64) {
	t.Helper()

	actual, ok := value.(float64)
	require.Truef(t, ok, "expected JSON number, got %T", value)
	require.InDelta(t, expected, actual, 0.000001)
}

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
		FileName:         "bad.xml",
		FilePath:         "/fake/path/bad.json",
		IsValid:          false,
		ValidationError:  errors.New("Incorrect characters '<' and '</>` found in file"),
		ValidationErrors: []string{"Incorrect characters '<' and '</>` found in file"},
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
		FileName:         "bad.json",
		FilePath:         "/fake/path/bad.json",
		IsValid:          false,
		ValidationError:  errors.New("error at line 3 column 10"),
		ValidationErrors: []string{"error at line 3 column 10"},
		StartLine:        3,
		StartColumn:      10,
	}
	reportLineOnly := Report{
		FileName:         "bad.yaml",
		FilePath:         "/fake/path/bad.yaml",
		IsValid:          false,
		ValidationError:  errors.New("yaml: line 5: mapping error"),
		ValidationErrors: []string{"yaml: line 5: mapping error"},
		StartLine:        5,
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

func Test_sarifReportMergesExternalRuns(t *testing.T) {
	tmpDir := t.TempDir()
	externalPath := filepath.Join(tmpDir, "external.sarif")
	require.NoError(t, os.WriteFile(externalPath, []byte(`{
  "version": "2.1.0",
  "$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "external-tool",
          "version": "1.2.3",
          "rules": [
            {
              "id": "external-rule",
              "shortDescription": {
                "text": "Preserved external rule"
              }
            }
          ]
        }
      },
      "results": [
        {
          "ruleId": "external-rule",
          "message": {
            "text": "external result"
          }
        }
      ]
    }
  ]
}`), 0o600))

	log, err := createSARIFReport([]Report{validReport}, SARIFMergeConfig{Files: []string{externalPath}})
	require.NoError(t, err)
	require.Len(t, log.Runs, 2)

	sarifBytes, err := json.Marshal(log)
	require.NoError(t, err)
	output := string(sarifBytes)
	assert.Contains(t, output, `"name":"config-file-validator"`)
	assert.Contains(t, output, `"name":"external-tool"`)
	assert.Contains(t, output, `"version":"1.2.3"`)
	assert.Contains(t, output, `"id":"external-rule"`)
	assert.Contains(t, output, `"ruleId":"external-rule"`)
}

func Test_sarifReportMergesCompatibleSARIFVersions(t *testing.T) {
	cases := []struct {
		name    string
		version string
		schema  string
	}{
		{
			name:    "patch version",
			version: "2.1.1",
			schema:  SARIFSchema,
		},
		{
			name:    "omitted version with SARIF schema",
			version: "",
			schema:  SARIFSchema,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			externalPath := filepath.Join(tmpDir, "external.sarif")
			input := map[string]any{
				"$schema": tc.schema,
				"runs": []map[string]any{
					{
						"tool": map[string]any{
							"driver": map[string]any{
								"name": "compatible-tool",
							},
						},
						"results": []any{},
					},
				},
			}
			if tc.version != "" {
				input["version"] = tc.version
			}
			data, err := json.Marshal(input)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(externalPath, data, 0o600))

			log, err := createSARIFReport([]Report{validReport}, SARIFMergeConfig{Files: []string{externalPath}})
			require.NoError(t, err)
			require.Len(t, log.Runs, 2)

			sarifBytes, err := json.Marshal(log)
			require.NoError(t, err)
			assert.Contains(t, string(sarifBytes), `"name":"compatible-tool"`)
		})
	}
}

func Test_sarifFilesInDirectoryRecursesAndSorts(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o700))

	nestedSARIF := filepath.Join(nestedDir, "a.sarif.json")
	rootSARIF := filepath.Join(tmpDir, "z.sarif")
	ignoredFile := filepath.Join(nestedDir, "ignored.json")
	require.NoError(t, os.WriteFile(nestedSARIF, []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(rootSARIF, []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(ignoredFile, []byte("{}"), 0o600))

	paths, err := sarifFilesInDirectory(tmpDir)
	require.NoError(t, err)
	require.Equal(t, []string{nestedSARIF, rootSARIF}, paths)
}

// --- Grouped stdout tests ---

func Test_stdoutGroupedReports(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return PrintSingleGroupStdout(map[string][]Report{
			"xml": mixedReports,
		})
	})
	require.NoError(t, err)
	assert.Contains(t, output, "xml\n")
	assert.Contains(t, output, "/fake/path/good.xml")
	assert.Contains(t, output, "Summary: 1 succeeded, 2 failed")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 2 failed")

	// With "Passed"/"Failed" keys to hit checkGroupsForPassFail returning false
	output, err = captureStdout(t, func() error {
		return PrintSingleGroupStdout(map[string][]Report{
			"Passed": {validReport},
			"Failed": {invalidReport},
		})
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Passed\n")
	assert.Contains(t, output, "Failed\n")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 1 failed")
	require.Equal(t, 1, strings.Count(output, "Summary:"))

	output, err = captureStdout(t, func() error {
		return PrintDoubleGroupStdout(map[string]map[string][]Report{
			"xml": {"directory": mixedReports},
		})
	})
	require.NoError(t, err)
	assert.Contains(t, output, "xml\n")
	assert.Contains(t, output, "    directory\n")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 2 failed")

	output, err = captureStdout(t, func() error {
		return PrintTripleGroupStdout(map[string]map[string]map[string][]Report{
			"xml": {"directory": {"pass-fail": mixedReports}},
		})
	})
	require.NoError(t, err)
	assert.Contains(t, output, "xml\n")
	assert.Contains(t, output, "    directory\n")
	assert.Contains(t, output, "        pass-fail\n")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 2 failed")

	groupTree := &GroupNode{
		Children: []*GroupNode{
			{
				Key: "xml",
				Children: []*GroupNode{
					{
						Key: "directory",
						Children: []*GroupNode{
							{
								Key: "Passed",
								Children: []*GroupNode{
									{Key: "Passed", Reports: []Report{validReport}},
								},
							},
						},
					},
				},
			},
		},
	}
	output, err = captureStdout(t, func() error {
		return PrintGroupStdout(groupTree)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "xml\n")
	assert.Contains(t, output, "    directory\n")
	assert.Contains(t, output, "/fake/path/good.xml")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 0 failed")
}

func Test_stdoutGroupedLeafRootDoesNotDuplicateSummary(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return PrintGroupStdout(&GroupNode{
			Reports: []Report{validReport, invalidReport},
		})
	})
	require.NoError(t, err)
	assert.Contains(t, output, "/fake/path/good.xml")
	assert.Contains(t, output, "/fake/path/bad.xml")
	assert.Contains(t, output, "Total Summary: 1 succeeded, 1 failed")
	require.Equal(t, 1, strings.Count(output, "Summary:"))
}

// --- Grouped JSON tests ---

func Test_jsonGroupedReports(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return PrintSingleGroupJSON(map[string][]Report{
			"xml": mixedReports,
		})
	})
	require.NoError(t, err)
	report := decodeJSONOutput(t, output)
	requireJSONNumber(t, report["totalPassed"], 1)
	requireJSONNumber(t, report["totalFailed"], 2)
	xmlFiles := requireJSONArray(t, requireJSONMap(t, report["files"])["xml"])
	require.Len(t, xmlFiles, 3)
	firstFile := requireJSONMap(t, xmlFiles[0])
	require.Equal(t, "/fake/path/good.xml", firstFile["path"])
	xmlSummaries := requireJSONArray(t, requireJSONMap(t, report["summary"])["xml"])
	require.Len(t, xmlSummaries, 1)
	xmlSummary := requireJSONMap(t, xmlSummaries[0])
	requireJSONNumber(t, xmlSummary["passed"], 1)
	requireJSONNumber(t, xmlSummary["failed"], 2)

	output, err = captureStdout(t, func() error {
		return PrintDoubleGroupJSON(map[string]map[string][]Report{
			"xml": {"directory": mixedReports},
		})
	})
	require.NoError(t, err)
	report = decodeJSONOutput(t, output)
	requireJSONNumber(t, report["totalPassed"], 1)
	requireJSONNumber(t, report["totalFailed"], 2)
	doubleFiles := requireJSONMap(t, requireJSONMap(t, report["files"])["xml"])
	require.Len(t, requireJSONArray(t, doubleFiles["directory"]), 3)
	doubleSummaries := requireJSONMap(t, requireJSONMap(t, report["summary"])["xml"])
	require.Len(t, requireJSONArray(t, doubleSummaries["directory"]), 1)

	output, err = captureStdout(t, func() error {
		return PrintTripleGroupJSON(map[string]map[string]map[string][]Report{
			"xml": {"directory": {"pass-fail": mixedReports}},
		})
	})
	require.NoError(t, err)
	report = decodeJSONOutput(t, output)
	requireJSONNumber(t, report["totalPassed"], 1)
	requireJSONNumber(t, report["totalFailed"], 2)
	tripleFiles := requireJSONMap(t, requireJSONMap(t, requireJSONMap(t, report["files"])["xml"])["directory"])
	require.Len(t, requireJSONArray(t, tripleFiles["pass-fail"]), 3)

	groupTree := &GroupNode{
		Children: []*GroupNode{
			{
				Key: "xml",
				Children: []*GroupNode{
					{
						Key: "directory",
						Children: []*GroupNode{
							{
								Key: "Passed",
								Children: []*GroupNode{
									{Key: "Passed", Reports: []Report{validReport}},
								},
							},
						},
					},
				},
			},
		},
	}
	output, err = captureStdout(t, func() error {
		return PrintGroupJSON(groupTree)
	})
	require.NoError(t, err)
	report = decodeJSONOutput(t, output)
	requireJSONNumber(t, report["totalPassed"], 1)
	requireJSONNumber(t, report["totalFailed"], 0)
	treeFiles := requireJSONMap(t, requireJSONMap(t, requireJSONMap(t, requireJSONMap(t, report["files"])["xml"])["directory"])["Passed"])
	require.Len(t, requireJSONArray(t, treeFiles["Passed"]), 1)
}

func Test_jsonGroupedLeafRootIncludesReports(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return PrintGroupJSON(&GroupNode{
			Reports: []Report{validReport, invalidReport},
		})
	})
	require.NoError(t, err)
	report := decodeJSONOutput(t, output)
	requireJSONNumber(t, report["totalPassed"], 1)
	requireJSONNumber(t, report["totalFailed"], 1)
	files := requireJSONArray(t, report["files"])
	require.Len(t, files, 2)
	firstFile := requireJSONMap(t, files[0])
	require.Equal(t, "/fake/path/good.xml", firstFile["path"])
	summaries := requireJSONArray(t, report["summary"])
	require.Len(t, summaries, 1)
	reportSummary := requireJSONMap(t, summaries[0])
	requireJSONNumber(t, reportSummary["passed"], 1)
	requireJSONNumber(t, reportSummary["failed"], 1)
}

// --- Reporter file output tests (shared pattern) ---

func Test_reporterFileOutput(t *testing.T) {
	report := Report{
		FileName: "good.json",
		FilePath: "/fake/path/good.json",
		IsValid:  true,
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
