package cli

import (
	"testing"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// TODO: Add tests

func Test_NoGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {},
		"test2": {},
		"test3": {},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err != nil {
			t.Errorf("An error was returned: %v", err)
		}

		if exitStatus != 0 {
			t.Errorf("Exit status was not 0")
		}
	}
}

func Test_SingleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"directory"},
		"test2": {"filetype"},
		"test3": {"pass-fail"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err != nil {
			t.Errorf("An error was returned: %v", err)
		}

		if exitStatus != 0 {
			t.Errorf("Exit status was not 0")
		}
	}
}

func Test_DoubleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"directory", "pass-fail"},
		"test2": {"filetype", "directory"},
		"test3": {"pass-fail", "filetype"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err != nil {
			t.Errorf("An error was returned: %v", err)
		}

		if exitStatus != 0 {
			t.Errorf("Exit status was not 0")
		}
	}
}

func Test_TripleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"directory", "pass-fail", "filetype"},
		"test2": {"filetype", "directory", "pass-fail"},
		"test3": {"pass-fail", "filetype", "directory"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err != nil {
			t.Errorf("An error was returned: %v", err)
		}

		if exitStatus != 0 {
			t.Errorf("Exit status was not 0")
		}
	}
}

func Test_IncorrectSingleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"bad"},
		"test2": {"more bad"},
		"test3": {"most bad"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err == nil {
			t.Errorf("An error was not returned")
		}

		if exitStatus != 1 {
			t.Errorf("Exit status was not 1")
		}
	}
}
func Test_IncorrectDoubleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"directory", "bad"},
		"test2": {"bad", "directory"},
		"test3": {"pass-fail", "bad"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err == nil {
			t.Errorf("An error was not returned")
		}

		if exitStatus != 1 {
			t.Errorf("Exit status was not 1")
		}
	}
}
func Test_IncorrectTripleGroupOutput(t *testing.T) {
	searchPath := "../../test"
	excludeDirs := []string{"subdir", "subdir2"}
	groupOutput := map[string][]string{
		"test":  {"bad", "pass-fail", "filetype"},
		"test2": {"filetype", "bad", "directory"},
		"test3": {"pass-fail", "filetype", "bad"},
	}
	stdoutReporter := reporter.StdoutReporter{}

	for i := range groupOutput {
		fsFinder := finder.FileSystemFinderInit(
			finder.WithPathRoots(searchPath),
			finder.WithExcludeDirs(excludeDirs),
		)
		cli := Init(
			WithFinder(fsFinder),
			WithReporter(stdoutReporter),
			WithGroupOutput(groupOutput[i]),
		)
		exitStatus, err := cli.Run()

		if err == nil {
			t.Errorf("An error was not returned")
		}

		if exitStatus != 1 {
			t.Errorf("Exit status was not 0")
		}
	}
}
