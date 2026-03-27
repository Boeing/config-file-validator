package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/internal/testhelper"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func Test_CLI(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml", "toml")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewStdoutReporter("")),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithMultipleReporters(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml")
	tmpOut := t.TempDir()

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(
			reporter.NewJSONReporter(tmpOut+"/result.json"),
			reporter.JunitReporter{},
		),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithFailedValidation(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "bad.json", testhelper.InvalidContent["json"])

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIBadPath(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("/bad/path"),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIWithGroup(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewStdoutReporter("")),
		WithGroupOutput([]string{"pass-fail", "directory"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIReportErr(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("./wrong/path")),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIWithFormattingCheckEnabled(t *testing.T) {
	dir := t.TempDir()
	file := testhelper.WriteFile(t, dir, "test.json", "{\n  \"key\": \"value\"\n}\n")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"json"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithFormattingDisabled(t *testing.T) {
	dir := t.TempDir()
	unformatted := []byte(`{"name":"test","values":[1,2,3]}`)
	file := testhelper.WriteFile(t, dir, "test.json", string(unformatted))

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)

	content, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, unformatted, content)
}

func Test_CLIFormattingWithInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	invalidJSON := []byte(`{"name":"test","invalid":}`)
	file := testhelper.WriteFile(t, dir, "test.json", string(invalidJSON))

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithFormatCheckTypes([]string{"json"}),
	)
	exitStatus, err := cli.Run()
	require.Equal(t, 1, exitStatus)
	require.NoError(t, err)

	content, err := os.ReadFile(file)
	require.NoError(t, err)
	require.True(t, bytes.Equal(invalidJSON, content))
}

func Test_CLIWithSchemaCheckEnabled(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "sarif")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "sarif")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	dir := t.TempDir()
	file := testhelper.WriteFile(t, dir, "bad.sarif", `{"version": "2.1.0", "runs": "not_an_array"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "yaml")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "json")

	err := os.Chmod(file, 0000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(file, 0600) }()

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIValidateCapabilitiesUnknownType(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
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
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIDoubleGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype", "directory"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLITripleGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype", "directory", "pass-fail"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}
