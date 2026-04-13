package configfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `
exclude-dirs = ["node_modules", ".git"]
exclude-file-types = ["csv"]
depth = 2
quiet = true
schemastore = true

[schema-map]
"**/package.json" = "schemas/pkg.json"

[type-map]
"**/inventory" = "ini"
`)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.Equal(t, []string{"node_modules", ".git"}, cfg.ExcludeDirs)
	require.Equal(t, []string{"csv"}, cfg.ExcludeFileTypes)
	require.Equal(t, 2, *cfg.Depth)
	require.True(t, *cfg.Quiet)
	require.True(t, *cfg.SchemaStore)
	require.Equal(t, "schemas/pkg.json", cfg.SchemaMap["**/package.json"])
	require.Equal(t, "ini", cfg.TypeMap["**/inventory"])
}

func TestLoadMinimal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `quiet = false`)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.False(t, *cfg.Quiet)
	require.Nil(t, cfg.Depth)
	require.Empty(t, cfg.ExcludeDirs)
}

func TestLoadEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, ``)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoadInvalidTOMLSyntax(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `quiet = [broken`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid TOML syntax")
}

func TestLoadUnknownKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `unknown-key = true`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestLoadWrongType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `quiet = "yes"`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestLoadDepthNegative(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `depth = -1`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestLoadInvalidGroupBy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `groupby = ["invalid"]`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	_, err := Load("/nonexistent/.cfv.toml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading config file")
}

func TestDiscoverInCurrentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `quiet = true`)

	path := Discover(dir)
	require.Equal(t, filepath.Join(dir, FileName), path)
}

func TestDiscoverInParentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `quiet = true`)

	child := filepath.Join(dir, "sub", "deep")
	require.NoError(t, os.MkdirAll(child, 0755))

	path := Discover(child)
	require.Equal(t, filepath.Join(dir, FileName), path)
}

func TestDiscoverNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := Discover(dir)
	require.Empty(t, path)
}

func TestLoadAllBooleans(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `
require-schema = true
no-schema = false
schemastore = true
globbing = false
`)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.True(t, *cfg.RequireSchema)
	require.False(t, *cfg.NoSchema)
	require.True(t, *cfg.SchemaStore)
	require.False(t, *cfg.Globbing)
}

func TestLoadReporter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `reporter = ["json:output.json", "standard"]`)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.Equal(t, []string{"json:output.json", "standard"}, cfg.Reporter)
}

func TestLoadSchemaStorePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `schemastore-path = "./schemastore"`)

	cfg, err := Load(filepath.Join(dir, FileName))
	require.NoError(t, err)
	require.Equal(t, "./schemastore", *cfg.SchemaStorePath)
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0600))
}

func TestLoadMultipleErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `
unknown1 = true
unknown2 = "bad"
`)

	_, err := Load(filepath.Join(dir, FileName))
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema validation failed")
}
