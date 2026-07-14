package yamlfmt

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// TestReindent verifies that printFormatted normalizes indent width.
func TestReindent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "4 space to 2 space",
			input:  "a:\n    b: 1\n    c: 2\n",
			width:  2,
			expect: "a:\n  b: 1\n  c: 2\n",
		},
		{
			name:   "2 space to 4 space",
			input:  "a:\n  b: 1\n  c: 2\n",
			width:  4,
			expect: "a:\n    b: 1\n    c: 2\n",
		},
		{
			name:   "nested 4 to 2",
			input:  "a:\n    b:\n        c: deep\n",
			width:  2,
			expect: "a:\n  b:\n    c: deep\n",
		},
		{
			name:   "already correct",
			input:  "a:\n  b: 1\n",
			width:  2,
			expect: "a:\n  b: 1\n",
		},
		{
			name:   "root keys no indent",
			input:  "a: 1\nb: 2\n",
			width:  2,
			expect: "a: 1\nb: 2\n",
		},
		{
			name:   "sequence",
			input:  "items:\n    - one\n    - two\n",
			width:  2,
			expect: "items:\n  - one\n  - two\n",
		},
		{
			name:   "preserves comments",
			input:  "# header\na:\n    # nested comment\n    b: 1\n",
			width:  2,
			expect: "# header\na:\n  # nested comment\n  b: 1\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokens := tokenize([]byte(tc.input))
			opts := formatter.Options{
				IndentWidth:  tc.width,
				FinalNewline: true,
				LineEnding:   formatter.LineEndingLF,
			}
			got := printFormatted(tokens, opts, []byte(tc.input))
			require.Equal(t, tc.expect, string(got), "reindent failed")
		})
	}
}

// TestReindentIdempotent verifies formatting the output again produces the same result.
func TestReindentIdempotent(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"a:\n    b:\n        c: deep\n    d: sibling\n",
		"items:\n    - one\n    - two\n",
		"# comment\nkey: value\n",
		"---\na: 1\nb: 2\n...\n",
	}

	opts := formatter.Options{
		IndentWidth:  2,
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
	}

	for _, input := range inputs {
		tokens := tokenize([]byte(input))
		first := printFormatted(tokens, opts, []byte(input))

		tokens2 := tokenize(first)
		second := printFormatted(tokens2, opts, first)

		require.Equal(t, string(first), string(second),
			"not idempotent.\nInput: %q\nFirst: %q\nSecond: %q", input, first, second)
	}
}

// TestSortKeys verifies key sorting within mapping scopes.
func TestSortKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple sort",
			input:  "c: 3\na: 1\nb: 2\n",
			expect: "a: 1\nb: 2\nc: 3\n",
		},
		{
			name:   "already sorted",
			input:  "a: 1\nb: 2\nc: 3\n",
			expect: "a: 1\nb: 2\nc: 3\n",
		},
		{
			name:   "with nested",
			input:  "z:\n  nested: value\na: simple\n",
			expect: "a: simple\nz:\n  nested: value\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokens := tokenize([]byte(tc.input))
			opts := formatter.Options{
				IndentWidth:  2,
				FinalNewline: true,
				LineEnding:   formatter.LineEndingLF,
				SortKeys:     true,
			}
			got := printFormatted(tokens, opts, []byte(tc.input))
			require.Equal(t, tc.expect, string(got), "sort keys failed")
		})
	}
}
