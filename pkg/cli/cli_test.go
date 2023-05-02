package cli

import (
	"testing"
)

func Test_CLI(t *testing.T) {
	cli := Init(
		"../../test", []string{"subdir", "subdir2"}, "standard")
	exitStatus, err := cli.Run()

	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 0 {
		t.Errorf("Exit status was not 0")
	}

	cli = Init("../../test", []string{"subdir", "subdir2"}, "json")
	exitStatus, err = cli.Run()

	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 0 {
		t.Errorf("Exit status was not 0")
	}
}

func Test_CLIWithFailedValidation(t *testing.T) {
	cli := Init(
		"../../test", []string{"subdir"}, "standard")
	exitStatus, err := cli.Run()

	if err != nil {
		t.Errorf("An error was returned: %v", err)
	}

	if exitStatus != 1 {
		t.Errorf("Exit status was not 1")
	}
}

func Test_CLIBadPath(t *testing.T) {
	cli := Init(
		"/bad/path", nil, "standard")
	exitStatus, err := cli.Run()

	if err == nil {
		t.Errorf("A nil error was returned")
	}

	if exitStatus == 0 {
		t.Errorf("Exit status was not 1")
	}
}
