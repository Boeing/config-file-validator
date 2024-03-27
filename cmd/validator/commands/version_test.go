package cmd

import (
	"testing"

	cmd "github.com/Boeing/config-file-validator/cmd/validator"
	"github.com/spf13/cobra"
)

func TestFlagVersion(t *testing.T) {
	var exitStatus int
	expectedExit := 0
	root := &cobra.Command{
		Use: "root",
		Run: func(c *cobra.Command, args []string) {
			exitStatus = cmd.ExecRoot(c)
		}}

	SetVersion("testing")
	root.AddCommand(versionCmd)

	args := []string{"version"}

	_, err := ExecuteTestHelper(t, root, args...)
	if err != nil {
		t.Error(err)
	}

	if expectedExit != exitStatus {
		t.Errorf("Wrong exit code, expected: %v, got: %v", expectedExit, exitStatus)
	}

}
