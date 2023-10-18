package cli

import (
	"testing"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func Test_CLI(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	stdoutReporter := reporter.StdoutReporter{}

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(searchPath),
		finder.WithExcludeDirs(excludeDirs),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporter(stdoutReporter),
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
