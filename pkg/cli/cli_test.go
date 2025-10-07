package cli

import (
	"testing"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

func Test_CLI(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{""}
	stdoutReporter := reporter.StdoutReporter{}

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporter(stdoutReporter),
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
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := []string{"pass-fail", "directory"}
	stdoutReporter := reporter.StdoutReporter{}

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporter(stdoutReporter),
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

func Test_CLIRepoertErr(t *testing.T) {
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
		WithReporter(jsonReporter),
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

func Test_CLI_IgnoreBadPklFileWhenBinaryNotFound(t *testing.T) {
	// Override the binary checker for this test and restore it afterward
	previousChecker := validator.SetPklBinaryChecker(func() bool {
		return false
	})
	defer validator.SetPklBinaryChecker(previousChecker)

	searchPath := "../../test/fixtures/subdir2/bad.pkl"
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
	)
	cli := Init(
		WithFinder(fsFinder),
	)
	exitStatus, err := cli.Run()
	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	// Since the pkl binary is not found, the bad pkl file should be ignored
	// So the exit status should be 0
	if exitStatus != 0 {
		t.Errorf("Expected exit status 0, but got: %d", exitStatus)
	}
}
