package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/internal/testhelper"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func Test_ValidGroupOutput(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml", "toml")

	cases := []struct {
		name        string
		groupOutput []string
	}{
		{"no grouping", []string{}},
		{"single directory", []string{"directory"}},
		{"single filetype", []string{"filetype"}},
		{"single pass-fail", []string{"pass-fail"}},
		{"single error-type", []string{"error-type"}},
		{"double directory,pass-fail", []string{"directory", "pass-fail"}},
		{"double filetype,directory", []string{"filetype", "directory"}},
		{"double pass-fail,filetype", []string{"pass-fail", "filetype"}},
		{"triple directory,pass-fail,filetype", []string{"directory", "pass-fail", "filetype"}},
		{"triple filetype,directory,pass-fail", []string{"filetype", "directory", "pass-fail"}},
		{"triple pass-fail,filetype,directory", []string{"pass-fail", "filetype", "directory"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsFinder := finder.FileSystemFinderInit(
				finder.WithPathRoots(dir),
			)
			cli := Init(
				WithFinder(fsFinder),
				WithReporters(reporter.NewStdoutReporter("")),
				WithGroupOutput(tc.groupOutput),
			)

			exitStatus, err := cli.Run()
			require.NoError(t, err)
			require.Equal(t, 0, exitStatus)
		})
	}
}

func Test_InvalidGroupOutput(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	cases := []struct {
		name        string
		groupOutput []string
	}{
		{"single bad", []string{"bad"}},
		{"single more bad", []string{"more bad"}},
		{"double directory,bad", []string{"directory", "bad"}},
		{"double bad,directory", []string{"bad", "directory"}},
		{"double pass-fail,bad", []string{"pass-fail", "bad"}},
		{"triple bad,pass-fail,filetype", []string{"bad", "pass-fail", "filetype"}},
		{"triple filetype,bad,directory", []string{"filetype", "bad", "directory"}},
		{"triple pass-fail,filetype,bad", []string{"pass-fail", "filetype", "bad"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsFinder := finder.FileSystemFinderInit(
				finder.WithPathRoots(dir),
			)
			cli := Init(
				WithFinder(fsFinder),
				WithReporters(reporter.NewStdoutReporter("")),
				WithGroupOutput(tc.groupOutput),
			)

			exitStatus, err := cli.Run()
			require.Error(t, err)
			require.Equal(t, 1, exitStatus)
		})
	}
}

func Test_GroupByDirectory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reports  []reporter.Report
		expected int
	}{
		{
			"unix paths",
			[]reporter.Report{
				{FileName: "test", FilePath: "test/test/test"},
				{FileName: "test2", FilePath: "test2/test2/test2"},
			},
			2,
		},
		{
			"windows paths",
			[]reporter.Report{
				{FileName: "test", FilePath: "test\\test\\test"},
				{FileName: "test2", FilePath: "test2\\test2\\test2"},
			},
			2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := GroupByDirectory(tc.reports)
			require.Len(t, result, tc.expected)
		})
	}
}

func Test_GroupByFileType(t *testing.T) {
	t.Parallel()

	reports := []reporter.Report{
		{FileName: "config.yml", FilePath: "/path/config.yml", IsValid: true},
		{FileName: "config.yaml", FilePath: "/path/config.yaml", IsValid: true},
		{FileName: "data.json", FilePath: "/path/data.json", IsValid: true},
	}

	result := GroupByFileType(reports)
	require.Len(t, result, 2)
	require.Len(t, result["yaml"], 2, "yml and yaml should be grouped together")
	require.Len(t, result["json"], 1)
}

func Test_GroupByErrorType(t *testing.T) {
	t.Parallel()

	reports := []reporter.Report{
		{FileName: "good.json", FilePath: "/path/good.json", IsValid: true},
		{FileName: "bad_syntax.json", FilePath: "/path/bad_syntax.json", IsValid: false, ErrorType: "syntax"},
		{FileName: "bad_schema.json", FilePath: "/path/bad_schema.json", IsValid: false, ErrorType: "schema"},
		{FileName: "bad_schema2.yaml", FilePath: "/path/bad_schema2.yaml", IsValid: false, ErrorType: "schema"},
	}

	result := GroupByErrorType(reports)
	require.Len(t, result["Passed"], 1)
	require.Len(t, result["syntax"], 1)
	require.Len(t, result["schema"], 2)
}

func Test_GroupByPassFail(t *testing.T) {
	t.Parallel()

	reports := []reporter.Report{
		{FileName: "good.json", FilePath: "/path/good.json", IsValid: true},
		{FileName: "good2.json", FilePath: "/path/good2.json", IsValid: true},
		{FileName: "bad.json", FilePath: "/path/bad.json", IsValid: false},
		{FileName: "bad2.json", FilePath: "/path/bad2.json", IsValid: false},
	}

	result := GroupByPassFail(reports)
	require.Len(t, result["Passed"], 2)
	require.Len(t, result["Failed"], 2)
}
