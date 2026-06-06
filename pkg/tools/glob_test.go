package tools

import "testing"

func TestIsGlobPattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input    string
		expected bool
	}{
		{"**/package.json", true},
		{"*.json", true},
		{"config.json", false},
		{"path/to/file", false},
		{"file[0-9].txt", true},
		{"what?", true},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			if got := IsGlobPattern(tc.input); got != tc.expected {
				t.Errorf("IsGlobPattern(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
