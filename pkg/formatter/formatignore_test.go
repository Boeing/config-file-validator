package formatter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

func TestFormatIgnores_NilNeverSkips(t *testing.T) {
	t.Parallel()
	var fi *formatter.FormatIgnores
	require.False(t, fi.ShouldSkipFormat("/some/file.json", "json"))
	require.False(t, fi.ShouldSkipFormat("/some/file.yaml", "yaml"))
	require.False(t, fi.ShouldSkipFormat("/some/file.toml", "toml"))
}

func TestFormatIgnores_PrettierMatchesJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "fixtures/**\n")

	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	absFixture := filepath.Join(dir, "fixtures", "test.json")
	absSrc := filepath.Join(dir, "src", "app.json")

	require.True(t, fi.ShouldSkipFormat(absFixture, "json"))
	require.True(t, fi.ShouldSkipFormat(absFixture, "jsonc"))
	require.False(t, fi.ShouldSkipFormat(absSrc, "json"))
}

func TestFormatIgnores_PrettierMatchesYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "*.gen.yaml\n")

	// No yamlfmt → prettier owns YAML.
	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	absGen := filepath.Join(dir, "api.gen.yaml")
	absNormal := filepath.Join(dir, "config.yaml")

	require.True(t, fi.ShouldSkipFormat(absGen, "yaml"))
	require.False(t, fi.ShouldSkipFormat(absNormal, "yaml"))
}

func TestFormatIgnores_PrettierNegation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "dist/**\n!dist/config.json\n")

	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	absDistOther := filepath.Join(dir, "dist", "bundle.json")
	absDistConfig := filepath.Join(dir, "dist", "config.json")

	require.True(t, fi.ShouldSkipFormat(absDistOther, "json"))
	require.False(t, fi.ShouldSkipFormat(absDistConfig, "json"))
}

func TestFormatIgnores_TaploExcludeMatchesToml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	taplo := &formatter.Taplo{
		Exclude:   []string{"tests/**"},
		ConfigDir: dir,
	}

	fi := formatter.BuildFormatIgnores("", taplo, nil)
	require.NotNil(t, fi)

	absTest := filepath.Join(dir, "tests", "fixture.toml")
	absSrc := filepath.Join(dir, "src", "config.toml")

	require.True(t, fi.ShouldSkipFormat(absTest, "toml"))
	require.False(t, fi.ShouldSkipFormat(absSrc, "toml"))
}

func TestFormatIgnores_TaploExcludeIgnoresOtherFormats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	taplo := &formatter.Taplo{
		Exclude:   []string{"tests/**"},
		ConfigDir: dir,
	}

	fi := formatter.BuildFormatIgnores("", taplo, nil)
	require.NotNil(t, fi)

	// Taplo exclude only applies to TOML.
	absTest := filepath.Join(dir, "tests", "fixture.json")
	require.False(t, fi.ShouldSkipFormat(absTest, "json"))
}

func TestFormatIgnores_YamlfmtExcludeMatchesYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	yamlfmt := &formatter.Yamlfmt{
		Exclude:   []string{"vendor/**"},
		ConfigDir: dir,
	}

	fi := formatter.BuildFormatIgnores("", nil, yamlfmt)
	require.NotNil(t, fi)

	absVendor := filepath.Join(dir, "vendor", "dep.yaml")
	absSrc := filepath.Join(dir, "src", "app.yaml")

	require.True(t, fi.ShouldSkipFormat(absVendor, "yaml"))
	require.False(t, fi.ShouldSkipFormat(absSrc, "yaml"))
}

func TestFormatIgnores_YamlfmtOwnershipOverridesPrettier(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// .prettierignore would match this file, but yamlfmt owns YAML.
	writeFile(t, dir, ".prettierignore", "generated/**\n")

	yamlfmt := &formatter.Yamlfmt{
		Exclude:   []string{}, // no yamlfmt excludes
		ConfigDir: dir,
	}

	fi := formatter.BuildFormatIgnores(dir, nil, yamlfmt)
	// yamlfmtOwnsYAML is true, so prettier patterns don't apply to YAML.
	absGen := filepath.Join(dir, "generated", "api.yaml")
	// Since yamlfmt has no excludes, this file is NOT skipped.
	require.False(t, fi.ShouldSkipFormat(absGen, "yaml"))
	// But it IS skipped for JSON (prettier still owns JSON).
	absGenJSON := filepath.Join(dir, "generated", "schema.json")
	require.True(t, fi.ShouldSkipFormat(absGenJSON, "json"))
}

func TestFormatIgnores_YamlfmtIgnoreFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// .yamlfmtignore file in same directory as .yamlfmt config.
	writeFile(t, dir, ".yamlfmtignore", "charts/**/templates/**\n")

	yamlfmt := &formatter.Yamlfmt{
		Exclude:   []string{"vendor/**"}, // from config
		ConfigDir: dir,
	}

	fi := formatter.BuildFormatIgnores("", nil, yamlfmt)
	require.NotNil(t, fi)

	// Matches config exclude.
	absVendor := filepath.Join(dir, "vendor", "dep.yaml")
	require.True(t, fi.ShouldSkipFormat(absVendor, "yaml"))

	// Matches .yamlfmtignore.
	absTemplate := filepath.Join(dir, "charts", "app", "templates", "deploy.yaml")
	require.True(t, fi.ShouldSkipFormat(absTemplate, "yaml"))

	// Matches neither.
	absSrc := filepath.Join(dir, "src", "config.yaml")
	require.False(t, fi.ShouldSkipFormat(absSrc, "yaml"))
}

func TestFormatIgnores_OutsideBaseDirNeverMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "**/*.json\n")

	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	// File outside the base dir should never match.
	absOutside := "/completely/different/path/file.json"
	require.False(t, fi.ShouldSkipFormat(absOutside, "json"))
}

func TestFormatIgnores_CommentAndBlankLinesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "# This is a comment\n\nfixtures/**\n\n# Another comment\n")

	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	absFixture := filepath.Join(dir, "fixtures", "test.json")
	require.True(t, fi.ShouldSkipFormat(absFixture, "json"))
}

func TestFormatIgnores_UnknownFormatNeverSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "**/*\n") // match everything

	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.NotNil(t, fi)

	absFile := filepath.Join(dir, "something.hcl")
	require.False(t, fi.ShouldSkipFormat(absFile, "hcl"))
	require.False(t, fi.ShouldSkipFormat(absFile, "xml"))
}

func TestFormatIgnores_NoPatterns_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No .prettierignore, no taplo exclude, no yamlfmt exclude.
	fi := formatter.BuildFormatIgnores(dir, nil, nil)
	require.Nil(t, fi)
}

func TestFindPrettierIgnore_WalksUp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".prettierignore", "dist/**\n")

	// Search from a subdirectory.
	subDir := filepath.Join(dir, "src", "components")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	patterns, baseDir := formatter.FindPrettierIgnore(subDir)
	require.NotNil(t, patterns)
	require.Equal(t, dir, baseDir)
}

func TestFindPrettierIgnore_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No .prettierignore anywhere.
	patterns, baseDir := formatter.FindPrettierIgnore(dir)
	require.Nil(t, patterns)
	require.Empty(t, baseDir)
}

func TestParseGitignorePatterns_EmptyReturnsNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, formatter.ParseGitignorePatterns([]byte("")))
	require.Nil(t, formatter.ParseGitignorePatterns([]byte("# only comments\n\n")))
}

// writeFile is a test helper to write a file with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
