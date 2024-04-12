package cmd

import (
	"bytes"
	"strings"
	"testing"

	cmd "github.com/Boeing/config-file-validator/cmd/validator"
	"github.com/spf13/cobra"
)

func ExecuteTestHelper(t *testing.T, c *cobra.Command, args ...string) (string, error) {
	t.Helper()

	buf := new(bytes.Buffer)
	c.SetOut(buf)
	c.SetErr(buf)
	c.SetArgs(args)

	err := c.Execute()
	return strings.TrimSpace(buf.String()), err
}

func Test_flags(t *testing.T) {
	// We manipulate the Args to set them up for the testcases
	cases := []struct {
		Name         string
		Args         []string
		ExpectedExit int
	}{
		{"blank", []string{}, 0},
		{"negative depth set", []string{"--depth", "-1", "--reporter", "standard"}, 1},
		{"flags set, wrong reporter", []string{"--exclude-dirs", "subdir", "--reporter", "wrong"}, 1},
		{"flags set, json reporter", []string{"--exclude-dirs", "subdir", "--reporter", "json"}, 0},
		{"flags set, junit reported", []string{"--exclude-dirs", "subdir", "--reporter", "junit"}, 0},
		{"bad path", []string{"/path/does/not/exit"}, 1},
		{"exclude file types set", []string{"--exclude-file-types", "json"}, 0},
		{"multiple paths", []string{"../../../test/fixtures/subdir/good.json", "../../../test/fixtures/good.json"}, 0},
		{"output set", []string{"--output", "../../../test/output", "--reporter", "json"}, 0},

		{"empty string output set", []string{"--output", "", "--reporter", "json", "."}, 0},
		{"wrong output set", []string{"--output", "/path/not/exist", "--reporter", "json", "."}, 1},
		{"incorrect group", []string{"--groupby", "badgroup"}, 1},
		{"correct group", []string{"--groupby", "directory"}, 0},
		{"grouped junit", []string{"--groupby", "directory", "--reporter", "junit", "."}, 1},
		{"groupby duplicate", []string{"--groupby", "directory,directory", "."}, 1},
		{"quiet flag", []string{"--quiet", "."}, 0},
	}

	var exitStatus int

	for _, tc := range cases {
		root := &cobra.Command{
			Use: "root",
			Run: func(c *cobra.Command, args []string) {
				exitStatus = cmd.ExecRoot(c, args)
			},
		}
		CmdFlags(root)

		t.Run(tc.Name, func(t *testing.T) {
			_, err := ExecuteTestHelper(t, root, tc.Args...)
			if err != nil {
				t.Error("ExecuteTestHelper: ", err)
			}

			if tc.ExpectedExit != exitStatus {
				t.Errorf("Wrong exit code, expected: %v, got: %v", tc.ExpectedExit, exitStatus)
			}
		})
	}

}
