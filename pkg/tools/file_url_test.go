package tools

import (
	"runtime"
	"testing"
)

func TestFileURL(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"/tmp/schema file#1.json": "file:///tmp/schema%20file%231.json",
	}
	if runtime.GOOS == "windows" {
		tests = map[string]string{
			`C:\work\schema file#1.json`:      "file:///C:/work/schema%20file%231.json",
			`\\server\share\schema file.json`: "file://server/share/schema%20file.json",
		}
	}

	for path, expected := range tests {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if got := FileURL(path); got != expected {
				t.Fatalf("FileURL(%q) = %q, want %q", path, got, expected)
			}
		})
	}
}
