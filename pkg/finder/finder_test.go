package finder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/internal/testhelper"
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/tools"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

func Test_fsFinder(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml", "toml")

	fsFinder := FileSystemFinderInit(WithPathRoots(dir))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.NotEmpty(t, files)
}

func Test_fsFinderExcludeDirs(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")
	sub := testhelper.CreateSubdir(t, dir, "excluded")
	testhelper.WriteFile(t, sub, "good.yaml", testhelper.ValidContent["yaml"])

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithExcludeDirs([]string{"excluded"}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func Test_fsFinderExcludeFileTypes(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "toml")

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithExcludeFileTypes([]string{"json"}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func Test_fsFinderWithDepth(t *testing.T) {
	// root/good.json, root/sub/good.yaml
	dir := testhelper.CreateFixtureDir(t, "json")
	sub := testhelper.CreateSubdir(t, dir, "sub")
	testhelper.WriteFile(t, sub, "good.yaml", testhelper.ValidContent["yaml"])

	cases := []struct {
		name     string
		depth    int
		expected int
	}{
		{"depth 0 finds only root", 0, 1},
		{"depth 1 finds both", 1, 2},
		{"depth 9 finds both", 9, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsFinder := FileSystemFinderInit(
				WithPathRoots(dir),
				WithDepth(tc.depth),
			)
			files, err := fsFinder.Find()
			require.NoError(t, err)
			require.Len(t, files, tc.expected)
		})
	}
}

func Test_fsFinderCustomTypes(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml")

	jsonOnly := filetype.FileType{
		Name:       "json",
		Extensions: tools.ArrToMap("json"),
		Validator:  validator.JSONValidator{},
	}
	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithFileTypes([]filetype.FileType{jsonOnly}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func Test_fsFinderKnownFiles(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, ".editorconfig", testhelper.ValidContent["editorconfig"])

	knownFileType := filetype.FileType{
		Name:       "editorconfig",
		Extensions: tools.ArrToMap("whatever"),
		KnownFiles: tools.ArrToMap(".editorconfig"),
		Validator:  validator.EditorConfigValidator{},
	}
	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithFileTypes([]filetype.FileType{knownFileType}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func Test_fsFinderPathNoExist(t *testing.T) {
	fsFinder := FileSystemFinderInit(WithPathRoots("/bad/path"))
	_, err := fsFinder.Find()
	require.Error(t, err)
}

func Test_fsFinderMultiplePaths(t *testing.T) {
	file1 := testhelper.CreateFixtureFile(t, "json")
	file2 := testhelper.CreateFixtureFile(t, "yaml")

	fsFinder := FileSystemFinderInit(WithPathRoots(file1, file2))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 2)
}

func Test_fsFinderDuplicateFiles(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")
	absPath, err := filepath.Abs(file)
	require.NoError(t, err)

	fsFinder := FileSystemFinderInit(WithPathRoots(file, absPath))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func Test_fsFinderCaseInsensitiveExtension(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "good.JSON", testhelper.ValidContent["json"])
	testhelper.WriteFile(t, dir, "good.YAml", testhelper.ValidContent["yaml"])

	fsFinder := FileSystemFinderInit(WithPathRoots(dir))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 2)
}

func Test_fsFinderBadSecondPath(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	fsFinder := FileSystemFinderInit(WithPathRoots(dir, "/bad/path"))
	_, err := fsFinder.Find()
	require.Error(t, err)
}

func Test_fsFinderWhitespacePaths(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"no whitespace", dir, false},
		{"leading whitespace", "  " + dir, false},
		{"trailing whitespace", dir + "  ", false},
		{"both", "  " + dir + "  ", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsFinder := FileSystemFinderInit(WithPathRoots(tc.path))
			files, err := fsFinder.Find()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, files)
			}
		})
	}
}

func Test_fsFinderWalkDirError(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := testhelper.CreateSubdir(t, tmpDir, "noperm")
	testhelper.WriteFile(t, subDir, "test.json", `{}`)

	err := os.Chmod(subDir, 0000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(subDir, 0755) }()

	fsFinder := FileSystemFinderInit(WithPathRoots(subDir))
	_, err = fsFinder.Find()
	require.Error(t, err)
}

func Test_fsFinderTypeOverrides(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "inventory", "[servers]\nhost=10.0.0.1\n")
	testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])

	iniType := filetype.FileType{
		Name:       "ini",
		Extensions: tools.ArrToMap("ini"),
		Validator:  validator.IniValidator{},
	}

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithTypeOverrides([]TypeOverride{
			{Pattern: "**/inventory", FileType: iniType},
		}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Verify the extensionless file was matched as ini
	var foundInventory bool
	for _, f := range files {
		if f.Name == "inventory" {
			require.Equal(t, "ini", f.FileType.Name)
			foundInventory = true
		}
	}
	require.True(t, foundInventory)
}

func Test_fsFinderTypeOverrideGlobDir(t *testing.T) {
	dir := t.TempDir()
	sub := testhelper.CreateSubdir(t, dir, "configs")
	testhelper.WriteFile(t, sub, "app", "key=value\n")
	testhelper.WriteFile(t, sub, "db", "key=value\n")

	propsType := filetype.FileType{
		Name:       "properties",
		Extensions: tools.ArrToMap("properties"),
		Validator:  validator.PropValidator{},
	}

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithTypeOverrides([]TypeOverride{
			{Pattern: "**/configs/*", FileType: propsType},
		}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 2)
	for _, f := range files {
		require.Equal(t, "properties", f.FileType.Name)
	}
}

func Test_fsFinderTypeOverrideNoMatch(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "randomfile", "some content")

	iniType := filetype.FileType{
		Name:       "ini",
		Extensions: tools.ArrToMap("ini"),
		Validator:  validator.IniValidator{},
	}

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithTypeOverrides([]TypeOverride{
			{Pattern: "**/inventory", FileType: iniType},
		}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Empty(t, files)
}

func Test_fsFinderTypeOverridePriority(t *testing.T) {
	dir := t.TempDir()
	// Write valid XML content with a .json extension
	testhelper.WriteFile(t, dir, "data.json", "<root><key>val</key></root>")

	xmlType := filetype.FileType{
		Name:       "xml",
		Extensions: tools.ArrToMap("xml"),
		Validator:  validator.XMLValidator{},
	}

	fsFinder := FileSystemFinderInit(
		WithPathRoots(dir),
		WithTypeOverrides([]TypeOverride{
			{Pattern: "**/*.json", FileType: xmlType},
		}),
	)
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "xml", files[0].FileType.Name)
}

func Benchmark_Finder(b *testing.B) {
	// Use a real directory for benchmarking
	dir, err := os.MkdirTemp("", "bench_finder")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for _, ext := range []string{"json", "yaml", "toml", "csv", "ini"} {
		if err := os.WriteFile(filepath.Join(dir, "file."+ext), []byte(`{}`), 0600); err != nil {
			b.Fatal(err)
		}
	}

	fsFinder := FileSystemFinderInit(WithPathRoots(dir))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = fsFinder.Find()
	}
}
