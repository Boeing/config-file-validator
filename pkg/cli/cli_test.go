package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func Test_CLI(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{""}
	stdoutReporter := reporter.NewStdoutReporter("")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(stdoutReporter),
		WithGroupOutput(groupOutput),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 0 {
		t.Error("Exit status was not 0")
	}
}

func Test_CLIWithMultipleReporters(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{""}
	tmpDir := t.TempDir()
	output := tmpDir + "/validator_result.json"
	reporters := []reporter.Reporter{
		reporter.NewJSONReporter(output),
		reporter.JunitReporter{},
	}

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporters...),
		WithGroupOutput(groupOutput),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 0 {
		t.Error("Exit status was not 0")
	}
}

func Test_CLIWithFailedValidation(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir"}
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 1 {
		t.Error("Exit status was not 1")
	}
}

func Test_CLIBadPath(t *testing.T) {
	searchPath := "/bad/path"
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
	)
	cli := Init(
		WithFinder(fsFinder),
	)
	exitStatus, err := cli.Run()

	if err == nil {
		t.Error("A nil error was returned")
	}

	if exitStatus == 0 {
		t.Error("Exit status was not 1")
	}
}

func Test_CLIWithGroup(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{"pass-fail", "directory"}
	stdoutReporter := reporter.NewStdoutReporter("")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(stdoutReporter),
		WithGroupOutput(groupOutput),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 0 {
		t.Error("Exit status was not 0")
	}
}

func Test_CLIReportErr(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{""}
	output := "./wrong/path"
	jsonReporter := reporter.NewJSONReporter(output)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(jsonReporter),
		WithGroupOutput(groupOutput),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error returned: %v", err)
	}

	if exitStatus == 0 {
		t.Errorf("should return err status code: %d", exitStatus)
	}
}

func Test_CLIWithFormattingCheckEnabled(t *testing.T) {
	// Create a temporary JSON file with unformatted content
	tempFile, err := os.CreateTemp("", "test_format_*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	unformattedBytes := []byte(`{"name":"test","values":[1,2,3],"nested":{"key":"value"}}`)
	err = os.WriteFile(tempFile.Name(), unformattedBytes, 0600)
	require.NoError(t, err)

	// Setup CLI with formatting enabled
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(tempFile.Name()),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"json"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)

	// Verify the file was formatted
	formattedContent, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)

	require.Equal(t, unformattedBytes, formattedContent)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithFormattingDisabled(t *testing.T) {
	// Create a temporary JSON file with unformatted content
	tempFile, err := os.CreateTemp("", "test_no_format_*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	unformatted := []byte(`{"name":"test","values":[1,2,3]}`)
	err = os.WriteFile(tempFile.Name(), unformatted, 0600)
	require.NoError(t, err)

	// Setup CLI without formatting enabled
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(tempFile.Name()),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)

	// Verify the file was NOT formatted (remains unchanged)
	content, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	require.Equal(t, unformatted, content)
}

func Test_CLIFormattingWithInvalidJSON(t *testing.T) {
	// Create a temporary file with invalid JSON
	tempFile, err := os.CreateTemp("", "test_invalid_*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	invalidJSON := []byte(`{"name":"test","invalid":}`)
	err = os.WriteFile(tempFile.Name(), invalidJSON, 0600)
	require.NoError(t, err)

	// Setup CLI with formatting enabled
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(tempFile.Name()),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"json"}),
	)

	exitStatus, err := cli.Run()
	// Should fail because validation fails on invalid JSON
	require.Equal(t, 1, exitStatus)
	require.NoError(t, err) // No CLI error, just validation failure

	// Verify the file was NOT changed due to validation failure
	content, err := os.ReadFile(tempFile.Name())
	require.NoError(t, err)
	require.True(t, bytes.Equal(invalidJSON, content))
}

func Test_CLIWithSchemaCheckEnabled(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.sarif"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaCheckTypes([]string{"sarif"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithSchemaCheckDisabled(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.sarif"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaCheckTypes([]string{}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithSchemaCheckUnsupportedType(t *testing.T) {
	// JSON doesn't implement SchemaValidator, should fail at startup
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaCheckTypes([]string{"json"}),
	)

	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIWithSchemaCheckInvalidFile(t *testing.T) {
	// Create a temp sarif file with valid syntax but invalid schema
	tempFile, err := os.CreateTemp("", "test_schema_*.sarif")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	invalidSchema := []byte(`{"version": "2.1.0", "runs": "not_an_array"}`)
	err = os.WriteFile(tempFile.Name(), invalidSchema, 0600)
	require.NoError(t, err)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(tempFile.Name()),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaCheckTypes([]string{"sarif"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIWithFormatCheckUnsupportedType(t *testing.T) {
	// YAML doesn't implement FormatValidator, should fail at startup
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.yaml"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"yaml"}),
	)

	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIWithQuiet(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithQuiet(true),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithUnreadableFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_unreadable_*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	err = os.WriteFile(tempFile.Name(), []byte(`{"key": "value"}`), 0600)
	require.NoError(t, err)

	// Remove read permissions
	err = os.Chmod(tempFile.Name(), 0000)
	require.NoError(t, err)
	defer os.Chmod(tempFile.Name(), 0600)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(tempFile.Name()),
	)
	cli := Init(
		WithFinder(fsFinder),
	)

	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIValidateCapabilitiesUnknownType(t *testing.T) {
	// A type name not in filetype.FileTypes should be skipped (continue branch)
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"nonexistent"}),
		WithSchemaCheckTypes([]string{"nonexistent"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISingleGroupJSON(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	jsonReporter := reporter.NewJSONReporter("")
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(jsonReporter),
		WithGroupOutput([]string{"filetype"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIDoubleGroupJSON(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	jsonReporter := reporter.NewJSONReporter("")
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(jsonReporter),
		WithGroupOutput([]string{"filetype", "directory"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLITripleGroupJSON(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("../../test/fixtures/good.json"),
	)
	jsonReporter := reporter.NewJSONReporter("")
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(jsonReporter),
		WithGroupOutput([]string{"filetype", "directory", "pass-fail"}),
	)

	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}
