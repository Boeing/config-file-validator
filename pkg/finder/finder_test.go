package finder

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v2/internal/testhelper"
	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/tools"
	"github.com/Boeing/config-file-validator/v2/pkg/validator"
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

func Test_fsFinderLinguistKnownFiles(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		content  string
		wantType string
	}{
		{"babelrc as jsonc", ".babelrc", `{"presets": ["env"]}`, "jsonc"},
		{"clang-format as yaml", ".clang-format", "BasedOnStyle: LLVM\n", "yaml"},
		{"gitconfig as ini", ".gitconfig", "[user]\nname=test\n", "ini"},
		{"pom.xml as xml", "pom.xml", "<project><modelVersion>4.0.0</modelVersion></project>", "xml"},
		{"Pipfile as toml", "Pipfile", "[packages]\n", "toml"},
		{"tsconfig.json as jsonc", "tsconfig.json", `{"compilerOptions": {}}`, "jsonc"},
		{"composer.lock as json", "composer.lock", `{"packages": []}`, "json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			testhelper.WriteFile(t, dir, tc.filename, tc.content)

			fsFinder := FileSystemFinderInit(WithPathRoots(dir))
			files, err := fsFinder.Find()
			require.NoError(t, err)
			require.Len(t, files, 1, "should find %s", tc.filename)
			require.Equal(t, tc.wantType, files[0].FileType.Name)
		})
	}
}

func Test_fsFinderLinguistKnownFileNotRecognized(t *testing.T) {
	// A random extensionless file should NOT be found
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "randomfile", "some content")

	fsFinder := FileSystemFinderInit(WithPathRoots(dir))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Empty(t, files)
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

	for b.Loop() {
		_, _ = fsFinder.Find()
	}
}

// initGitRepo creates a temp directory with a real git repo and returns the path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	return dir
}

func fileNames(files []FileMetadata) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func Test_fsFinderGitignore(t *testing.T) {
	tests := []struct {
		name     string
		useRepo  bool
		setup    func(t *testing.T, dir string)
		pathRoot func(dir string) string // defaults to dir if nil
		opts     []FSFinderOptions
		expected []string
	}{
		{
			name:    "root gitignore excludes files",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "ignored.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "ignored.json\n")
			},
			expected: []string{"good.json"},
		},
		{
			name:    "directory pattern skips entire dir",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "keep.json", testhelper.ValidContent["json"])
				sub := testhelper.CreateSubdir(t, dir, "build")
				testhelper.WriteFile(t, sub, "skip.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "build/\n")
			},
			expected: []string{"keep.json"},
		},
		{
			name:    "nested gitignore file",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "root.json", testhelper.ValidContent["json"])
				sub := testhelper.CreateSubdir(t, dir, "sub")
				testhelper.WriteFile(t, sub, "keep.yaml", testhelper.ValidContent["yaml"])
				testhelper.WriteFile(t, sub, "local.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, sub, ".gitignore", "local.json\n")
			},
			expected: []string{"root.json", "keep.yaml"},
		},
		{
			name:    "git info exclude",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "keep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "secret.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, filepath.Join(dir, ".git", "info"), "exclude", "secret.json\n")
			},
			expected: []string{"keep.json"},
		},
		{
			name:    "no-op without git repo",
			useRepo: false,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "a.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "b.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "b.json\n")
			},
			expected: []string{"a.json", "b.json"},
		},
		{
			name:    "negation pattern",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "a.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "b.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "important.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "*.json\n!important.json\n")
			},
			expected: []string{"important.json"},
		},
		{
			name:    "doublestar pattern",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "keep.json", testhelper.ValidContent["json"])
				sub := testhelper.CreateSubdir(t, dir, "a")
				deep := testhelper.CreateSubdir(t, sub, "b")
				testhelper.WriteFile(t, deep, "debug.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, deep, "keep.yaml", testhelper.ValidContent["yaml"])
				testhelper.WriteFile(t, dir, ".gitignore", "**/debug.json\n")
			},
			expected: []string{"keep.json", "keep.yaml"},
		},
		{
			name:    "additive with exclude-dirs",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "app.json", testhelper.ValidContent["json"])
				vendor := testhelper.CreateSubdir(t, dir, "vendor")
				tests := testhelper.CreateSubdir(t, dir, "tests")
				testhelper.WriteFile(t, vendor, "dep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, tests, "test.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "vendor/\n")
			},
			opts:     []FSFinderOptions{WithExcludeDirs([]string{"tests"})},
			expected: []string{"app.json"},
		},
		{
			name:    "subdirectory search path inherits parent gitignore",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				sub := testhelper.CreateSubdir(t, dir, "sub")
				testhelper.WriteFile(t, sub, "keep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, sub, "drop.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "drop.json\n")
			},
			pathRoot: func(dir string) string { return filepath.Join(dir, "sub") },
			expected: []string{"keep.json"},
		},
		{
			name:    "intermediate directory gitignore",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mid := testhelper.CreateSubdir(t, dir, "mid")
				leaf := testhelper.CreateSubdir(t, mid, "leaf")
				testhelper.WriteFile(t, leaf, "keep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, leaf, "drop.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, mid, ".gitignore", "drop.json\n")
			},
			pathRoot: func(dir string) string { return filepath.Join(dir, "mid", "leaf") },
			expected: []string{"keep.json"},
		},
		{
			name:    "works with depth flag",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "root.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "drop.json", testhelper.ValidContent["json"])
				sub := testhelper.CreateSubdir(t, dir, "sub")
				testhelper.WriteFile(t, sub, "deep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "drop.json\n")
			},
			opts:     []FSFinderOptions{WithDepth(0)},
			expected: []string{"root.json"},
		},
		{
			name:    "handles BOM in gitignore",
			useRepo: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				testhelper.WriteFile(t, dir, "keep.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, "drop.json", testhelper.ValidContent["json"])
				testhelper.WriteFile(t, dir, ".gitignore", "\xef\xbb\xbfdrop.json\n")
			},
			expected: []string{"keep.json"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dir string
			if tc.useRepo {
				dir = initGitRepo(t)
			} else {
				dir = t.TempDir()
			}
			tc.setup(t, dir)

			root := dir
			if tc.pathRoot != nil {
				root = tc.pathRoot(dir)
			}
			opts := append([]FSFinderOptions{WithPathRoots(root), WithGitignore(true)}, tc.opts...)
			fsFinder := FileSystemFinderInit(opts...)
			files, err := fsFinder.Find()
			require.NoError(t, err)

			names := fileNames(files)
			require.Len(t, names, len(tc.expected))
			for _, exp := range tc.expected {
				require.Contains(t, names, exp)
			}
		})
	}
}

func Test_fsFinderGitignoreDisabled(t *testing.T) {
	dir := initGitRepo(t)
	testhelper.WriteFile(t, dir, "a.json", testhelper.ValidContent["json"])
	testhelper.WriteFile(t, dir, "b.json", testhelper.ValidContent["json"])
	testhelper.WriteFile(t, dir, ".gitignore", "b.json\n")

	fsFinder := FileSystemFinderInit(WithPathRoots(dir))
	files, err := fsFinder.Find()
	require.NoError(t, err)
	require.Len(t, files, 2, "without WithGitignore, .gitignore should have no effect")
}
