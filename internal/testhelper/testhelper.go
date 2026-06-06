package testhelper

import (
	"os"
	"path/filepath"
	"testing"
)

// Minimal valid content for each file type
var ValidContent = map[string]string{
	"json":         `{"key": "value"}`,
	"yaml":         "key: value\n",
	"yml":          "key: value\n",
	"toml":         "key = \"value\"\n",
	"csv":          "a,b,c\n1,2,3\n",
	"ini":          "[section]\nkey=value\n",
	"env":          "KEY=VALUE\n",
	"hcl":          "key = \"value\"\n",
	"xml":          "<root><key>value</key></root>",
	"properties":   "key=value\n",
	"hocon":        "key = value\n",
	"editorconfig": "root = true\n",
	"plist": `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>k</key><string>v</string></dict></plist>`,
	"sarif": `{
		"version": "2.1.0",
		"$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
		"runs": [{"tool": {"driver": {"name": "test", "language": "en"}}, "results": [], "language": "en", "newlineSequences": ["\n"]}]
	}`,
	"jsonc":  "// comment\n{\"key\": \"value\"}\n",
	"tf":     "key = \"value\"\n",
	"tfvars": "key = \"value\"\n",
}

var InvalidContent = map[string]string{
	"json":  `{"bad": }`,
	"yaml":  "a: b\nc: d:::::::::::::::\n",
	"toml":  "key = 123__456\n",
	"csv":   "This string has a \\\" in it",
	"jsonc": `{"bad": }`,
}

// CreateFixtureDir creates a temp directory with valid config files for the given extensions.
// Returns the path to the temp directory.
func CreateFixtureDir(t *testing.T, extensions ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, ext := range extensions {
		content, ok := ValidContent[ext]
		if !ok {
			t.Fatalf("no valid content defined for extension %q", ext)
		}
		WriteFile(t, dir, "good."+ext, content)
	}
	return dir
}

// CreateFixtureFile creates a single temp file with the given extension and valid content.
// Returns the path to the file.
func CreateFixtureFile(t *testing.T, ext string) string {
	t.Helper()
	dir := t.TempDir()
	content, ok := ValidContent[ext]
	if !ok {
		t.Fatalf("no valid content defined for extension %q", ext)
	}
	return WriteFile(t, dir, "good."+ext, content)
}

// WriteFile writes a file with the given name and content in dir. Returns the full path.
func WriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// CreateSubdir creates a subdirectory under dir and returns its path.
func CreateSubdir(t *testing.T, dir, name string) string {
	t.Helper()
	sub := filepath.Join(dir, name)
	err := os.Mkdir(sub, 0755)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}
