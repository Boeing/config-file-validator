package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func Test_CLI(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2", "bad-sarif"}
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
		t.Errorf("Exit status was not 0")
	}
}

func Test_CLIWithMultipleReporters(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2", "bad-sarif"}
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
		t.Errorf("Exit status was not 0")
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
		t.Errorf("Exit status was not 1")
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
		t.Errorf("A nil error was returned")
	}

	if exitStatus == 0 {
		t.Errorf("Exit status was not 1")
	}
}

func Test_CLIWithGroup(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2", "bad-sarif"}
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
		t.Errorf("Exit status was not 0")
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
