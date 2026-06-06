package gojust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAllTestdata(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".just") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if len(content) == 0 {
				t.Skip("empty file")
			}
			jf, err := Parse(content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			t.Logf("Parsed: %d recipes, %d assignments, %d settings, %d aliases, %d imports, %d modules",
				len(jf.Recipes), len(jf.Assignments), len(jf.Settings),
				len(jf.Aliases), len(jf.Imports), len(jf.Modules))
		})
	}
}
