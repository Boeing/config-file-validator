package schemastore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupTestBundle(t *testing.T, entries []catalogEntry) string {
	t.Helper()
	dir := t.TempDir()

	catalogDir := filepath.Join(dir, "src", "api", "json")
	require.NoError(t, os.MkdirAll(catalogDir, 0755))

	schemaDir := filepath.Join(dir, "src", "schemas", "json")
	require.NoError(t, os.MkdirAll(schemaDir, 0755))

	cat := catalog{Schemas: entries}
	data, err := json.Marshal(cat)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(catalogDir, "catalog.json"), data, 0600))

	return dir
}

func writeSchema(t *testing.T, bundlePath, filename, content string) string {
	t.Helper()
	p := filepath.Join(bundlePath, "src", "schemas", "json", filename)
	require.NoError(t, os.WriteFile(p, []byte(content), 0600))
	return p
}

func TestOpen(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"package.json"}, URL: "https://www.schemastore.org/package.json"},
		{FileMatch: []string{"*.hcl"}, URL: "https://www.schemastore.org/hcl.json"},         // unsupported ext
		{FileMatch: []string{"tsconfig*.json"}, URL: "https://www.schemastore.org/ts.json"}, // supported
	}
	dir := setupTestBundle(t, entries)
	store, err := Open(dir)
	require.NoError(t, err)
	// hcl entry should be filtered out
	require.Len(t, store.entries, 2)
}

func TestOpenMissingCatalog(t *testing.T) {
	t.Parallel()
	_, err := Open(t.TempDir())
	require.Error(t, err)
}

func TestLookupExactMatch(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"package.json"}, URL: "https://www.schemastore.org/package.json"},
	}
	dir := setupTestBundle(t, entries)
	writeSchema(t, dir, "package.json", `{"type":"object"}`)

	store, err := Open(dir)
	require.NoError(t, err)

	path, found := store.Lookup("/some/project/package.json")
	require.True(t, found)
	require.Contains(t, path, "package.json")
}

func TestLookupGlobMatch(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"**/.github/workflows/*.yml"}, URL: "https://www.schemastore.org/github-workflow.json"},
	}
	dir := setupTestBundle(t, entries)
	writeSchema(t, dir, "github-workflow.json", `{"type":"object"}`)

	store, err := Open(dir)
	require.NoError(t, err)

	path, found := store.Lookup("/repo/.github/workflows/ci.yml")
	require.True(t, found)
	require.Contains(t, path, "github-workflow.json")
}

func TestLookupNoMatch(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"package.json"}, URL: "https://www.schemastore.org/package.json"},
	}
	dir := setupTestBundle(t, entries)
	writeSchema(t, dir, "package.json", `{"type":"object"}`)

	store, err := Open(dir)
	require.NoError(t, err)

	_, found := store.Lookup("/some/project/tsconfig.json")
	require.False(t, found)
}

func TestLookupExternalURLSkipped(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"foo.json"}, URL: "https://example.com/foo.json"},
	}
	dir := setupTestBundle(t, entries)

	store, err := Open(dir)
	require.NoError(t, err)

	_, found := store.Lookup("/project/foo.json")
	require.False(t, found)
}

func TestLookupMissingSchemaFile(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"package.json"}, URL: "https://www.schemastore.org/package.json"},
	}
	dir := setupTestBundle(t, entries)
	// Don't write the schema file

	store, err := Open(dir)
	require.NoError(t, err)

	_, found := store.Lookup("/project/package.json")
	require.False(t, found)
}

func TestLookupGlobStarJson(t *testing.T) {
	t.Parallel()
	entries := []catalogEntry{
		{FileMatch: []string{"tsconfig*.json"}, URL: "https://www.schemastore.org/tsconfig.json"},
	}
	dir := setupTestBundle(t, entries)
	writeSchema(t, dir, "tsconfig.json", `{"type":"object"}`)

	store, err := Open(dir)
	require.NoError(t, err)

	path, found := store.Lookup("tsconfig.build.json")
	require.True(t, found)
	require.Contains(t, path, "tsconfig.json")
}
