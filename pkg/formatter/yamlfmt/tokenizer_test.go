package yamlfmt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTokenizeLossless verifies the fundamental invariant:
// concatenating all Token.Raw fields reproduces the input exactly.
func TestTokenizeLossless(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"newline only", "\n"},
		{"blank lines", "\n\n\n"},
		{"simple kv", "key: value\n"},
		{"comment", "# comment\n"},
		{"indented", "  indented: yes\n"},
		{"two keys", "a: 1\nb: 2\n"},
		{"doc start", "---\nfoo: bar\n"},
		{"doc end", "foo: bar\n...\n"},
		{"directive", "%YAML 1.2\n---\nk: v\n"},
		{"mixed", "# header\n\na: 1\n  # indented comment\nb: 2\n"},
		{"crlf", "key: value\r\n"},
		{"no final newline", "key: value"},
		{"tabs in value", "key: \tvalue\n"},
		{"multi-doc", "---\na: 1\n...\n---\nb: 2\n"},
		{"spaces only line", "   \n"},
		{"doc marker with suffix", "--- # comment\n"},
		// Block scalars
		{"literal block", "key: |\n  line1\n  line2\n"},
		{"folded block", "key: >\n  line1\n  line2\n"},
		{"block with indicator", "key: |2\n  content\n"},
		{"block with chomp", "key: |+\n  keep\n  trailing\n\n"},
		{"block with chomp strip", "key: |-\n  strip\n"},
		{"block empty lines", "key: |\n  line1\n\n  line3\n"},
		{"block ends at lower indent", "a: |\n  content\nb: value\n"},
		{"block at root", "|\n  root block\n"},
		{"block with comment", "key: | # comment\n  content\n"},
		{"block before doc end", "key: |\n  content\n...\n"},
		{"not a block - pipe in value", "key: a | b\n"},
		// Flow collections
		{"flow mapping", "key: {a: 1, b: 2}\n"},
		{"flow sequence", "key: [1, 2, 3]\n"},
		{"nested flow", "key: {a: [1, 2], b: {c: 3}}\n"},
		{"flow with quotes", `key: {"a]b": 1, 'c}d': 2}` + "\n"},
		{"flow multiline", "key: {\n  a: 1,\n  b: 2\n}\n"},
		{"flow with escape", `key: ["a\"b", 'c''d']` + "\n"},
		// Key/colon/value
		{"key colon value", "name: John\n"},
		{"key colon no value", "empty:\n"},
		{"quoted key", `"key with spaces": value` + "\n"},
		{"single quoted key", "'key': value\n"},
		{"colon in value", "url: http://example.com\n"},
		{"colon no space not separator", "time: 12:30:00\n"},
		// Sequence entries
		{"dash entry", "- item\n"},
		{"dash with key", "- name: value\n"},
		{"dash only", "- \n"},
		// Anchors and aliases
		{"anchor", "&anchor value\n"},
		{"alias", "*ref\n"},
		{"anchor on key", "&id key: value\n"},
		// Tags
		{"tag", "!!str value\n"},
		{"tag on key", "!custom key: value\n"},
		{"verbatim tag", "!<tag:yaml.org,2002:str> value\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokens := tokenize([]byte(tc.src))
			var reconstructed []byte
			for _, tok := range tokens {
				reconstructed = append(reconstructed, tok.Raw...)
			}
			require.Equal(t, tc.src, string(reconstructed),
				"losslessness violated: tokens don't reconstruct input")
		})
	}
}

// FuzzTokenizeLossless feeds arbitrary bytes to tokenize and verifies:
// - No panics on any input
// - Concatenated Token.Raw fields always equal the input (losslessness)
func FuzzTokenizeLossless(f *testing.F) {
	// Seed corpus with representative YAML constructs.
	f.Add([]byte("key: value\n"))
	f.Add([]byte("# comment\na: 1\nb: 2\n"))
	f.Add([]byte("---\nfoo: bar\n...\n"))
	f.Add([]byte("  indented: yes\n"))
	f.Add([]byte("%YAML 1.2\n---\nk: v\n"))
	f.Add([]byte("key: |\n  line1\n  line2\n"))
	f.Add([]byte("key: >\n  folded\n"))
	f.Add([]byte("key: {a: 1, b: [2, 3]}\n"))
	f.Add([]byte("- item1\n- item2\n"))
	f.Add([]byte("&anchor key: *alias\n"))
	f.Add([]byte("!!str value\n"))
	f.Add([]byte("\"quoted: key\": value\n"))
	f.Add([]byte("'single: key': value\n"))
	f.Add([]byte("multi:\n  nested:\n    deep: value\n"))
	f.Add([]byte("url: http://example.com:8080/path\n"))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0xFF, 0xFE})
	f.Add([]byte("|\n"))
	f.Add([]byte(">\n"))
	f.Add([]byte("---\n...\n"))
	f.Add([]byte("- |\n  block in seq\n"))
	f.Add([]byte("key: |2\n  explicit indent\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tokens := tokenize(data)
		var reconstructed []byte
		for _, tok := range tokens {
			reconstructed = append(reconstructed, tok.Raw...)
		}
		if string(reconstructed) != string(data) {
			t.Fatalf("losslessness violated.\nInput:  %q\nGot:    %q", data, reconstructed)
		}
	})
}
