package tools

import (
	"testing"
)

func TestPathToFileURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		volume string
		want   string
	}{
		{
			name: "unix",
			path: "/tmp/schema file#1.json",
			want: "file:///tmp/schema%20file%231.json",
		},
		{
			name:   "drive letter",
			path:   `C:\work\schema file#1.json`,
			volume: "C:",
			want:   "file:///C:/work/schema%20file%231.json",
		},
		{
			name:   "UNC path",
			path:   `\\server\share\schema file.json`,
			volume: `\\server\share`,
			want:   "file://server/share/schema%20file.json",
		},
		{
			name:   "UNC host root",
			path:   `\\server`,
			volume: `\\server`,
			want:   "file://server/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := pathToFileURL(test.path, test.volume); got != test.want {
				t.Fatalf("pathToFileURL(%q, %q) = %q, want %q", test.path, test.volume, got, test.want)
			}
		})
	}
}
