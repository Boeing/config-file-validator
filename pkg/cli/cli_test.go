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
	output := "../../test/output/validator_result.json"
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

	err = os.Remove(output)
	require.NoError(t, err)
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
