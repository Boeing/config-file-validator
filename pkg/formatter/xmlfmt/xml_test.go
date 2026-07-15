package xmlfmt_test

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
)

var f = xmlfmt.Formatter{}
var defaultOpts = xmlfmt.DefaultOptions()

// TestFixtures runs all .input.xml -> .expected.xml fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.xml")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.xml")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expected := strings.Replace(input, ".input.", ".expected.", 1)

			src, err := os.ReadFile(input)
			require.NoError(t, err)
			want, err := os.ReadFile(expected)
			require.NoError(t, err)

			optsFile := "testdata/" + name + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			got, err := f.Format(src, opts)
			require.NoError(t, err, "Format(%s) should not error", name)
			require.Equal(t, string(want), string(got), "unexpected output for %s", name)
		})
	}
}

// TestIdempotency verifies Format(Format(x)) == Format(x) for all fixtures.
func TestIdempotency(t *testing.T) {
	t.Parallel()
	expected, err := filepath.Glob("testdata/*.expected.xml")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.xml")
			optsFile := "testdata/" + baseName + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			first, err := f.Format(src, opts)
			require.NoError(t, err)
			second, err := f.Format(first, opts)
			require.NoError(t, err)

			require.Equal(t, string(first), string(second),
				"Format is not idempotent for %s", name)
		})
	}
}

// TestInvalidXMLReturnsError verifies that unparseable input returns an error.
func TestInvalidXMLReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"unclosed tag", "<root><item></root>"},
		{"invalid char", "<root>\x00</root>"},
		{"empty input", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid XML: %s", tc.src)
		})
	}
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("<root><item>hello</item></root>")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

// TestCommentPreservation verifies that XML comments survive formatting.
func TestCommentPreservation(t *testing.T) {
	t.Parallel()
	src := []byte(`<root><!-- important comment --><item>value</item></root>`)
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "<!-- important comment -->",
		"comment was not preserved")
}

// TestMixedContentPreservation proves that mixed-content elements (containing
// both text and child elements) are preserved inline without formatting
// whitespace being inserted between them.
func TestMixedContentPreservation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains []string // substrings that MUST appear in output (inline content preserved)
		excludes []string // substrings that MUST NOT appear (no newlines injected into mixed content)
	}{
		{
			name:     "inline_emphasis",
			input:    `<doc><p>Hello <b>world</b>!</p></doc>`,
			contains: []string{"<p>Hello <b>world</b>!</p>"},
			excludes: []string{"<p>\n", "\n<b>", "</b>\n"},
		},
		{
			name:     "multiple_inline_elements",
			input:    `<doc><p>Start <em>middle</em> and <code>end</code>.</p></doc>`,
			contains: []string{"<p>Start <em>middle</em> and <code>end</code>.</p>"},
		},
		{
			name:     "text_only_element_stays_inline",
			input:    `<root><name>just text</name></root>`,
			contains: []string{"<name>just text</name>"},
		},
		{
			name:     "self_closing_in_mixed_content",
			input:    `<doc><p>Text <br/> more text</p></doc>`,
			contains: []string{"<p>Text <br/> more text</p>"},
		},
		{
			name:     "structure_only_gets_formatted",
			input:    `<root><a><b><c>val</c></b></a></root>`,
			contains: []string{"<root>\n", "  <a>\n", "    <b>\n", "      <c>val</c>"},
		},
		{
			name:     "mixed_at_various_depths",
			input:    `<root><outer><p>text <b>bold</b> text</p></outer></root>`,
			contains: []string{"<p>text <b>bold</b> text</p>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Format([]byte(tc.input), defaultOpts)
			require.NoError(t, err)
			output := string(got)

			for _, s := range tc.contains {
				require.Contains(t, output, s,
					"expected inline content preserved:\ninput:  %s\noutput: %s", tc.input, output)
			}
			for _, s := range tc.excludes {
				require.NotContains(t, output, s,
					"unexpected formatting whitespace in mixed content:\ninput:  %s\noutput: %s", tc.input, output)
			}

			// Idempotency check.
			got2, err := f.Format(got, defaultOpts)
			require.NoError(t, err)
			require.Equal(t, string(got), string(got2),
				"must be idempotent:\nfirst:  %q\nsecond: %q", got, got2)
		})
	}
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid XML
	f.Add([]byte("<root><item>hello</item></root>"))
	f.Add([]byte(`<?xml version="1.0"?><r a="1"><c/></r>`))
	f.Add([]byte("<!-- comment --><root/>"))
	f.Add([]byte("<root>\n  <a>\n    <b>text</b>\n  </a>\n</root>"))
	f.Add([]byte(""))
	f.Add([]byte("not xml at all"))
	f.Add([]byte("<unclosed>"))
	f.Add([]byte{0x00, 0xFF, 0xFE})

	fmtr := xmlfmt.Formatter{}
	opts := xmlfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmtr.Format(data, opts)
		if err != nil {
			// Error is fine — just ensure no panic
			return
		}

		// If Format succeeded, the output must also parse successfully
		result2, err2 := fmtr.Format(result, opts)
		if err2 != nil {
			t.Fatalf("Format succeeded on input but failed on its own output.\nInput: %q\nOutput: %q\nError: %v",
				data, result, err2)
		}

		// Idempotency: Format(Format(x)) == Format(x)
		if string(result) != string(result2) {
			t.Fatalf("Format is not idempotent.\nFirst:  %q\nSecond: %q", result, result2)
		}
	})
}

func FuzzFormatWithOptions(f *testing.F) {
	f.Add([]byte("<?xml version=\"1.0\"?>\n<root><child/></root>\n"), byte(0))
	f.Add([]byte("<?xml version=\"1.0\"?>\n<r><a x=\"1\"/><b y=\"2\"/></r>\n"), byte(1))
	f.Add([]byte("<?xml version=\"1.0\"?>\n<r>\n  <c>text</c>\n</r>\n"), byte(4))

	fmtr := xmlfmt.Formatter{}
	f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
		opts := xmlfmt.DefaultOptions()
		if optByte&0x01 != 0 {
			opts.IndentWidth = 4
		}
		if optByte&0x02 != 0 {
			opts.FinalNewline = false
		}
		if optByte&0x04 != 0 {
			opts.XMLSelfClosingSpace = true
		}

		result, err := fmtr.Format(data, opts)
		if err != nil {
			return
		}

		result2, err := fmtr.Format(result, opts)
		if err != nil {
			t.Fatalf("second format failed: %v\nfirst: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent with opts=%08b:\ninput:  %q\nfirst:  %q\nsecond: %q", optByte, data, result, result2)
		}

		// Semantic equivalence: both must decode to same XML tokens.
		// Use a simplified check: both must be valid XML with same element structure.
		origDec := xml.NewDecoder(bytes.NewReader(data))
		fmtDec := xml.NewDecoder(bytes.NewReader(result))
		var origElements, fmtElements []string
		for {
			tok, err := origDec.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok {
				origElements = append(origElements, se.Name.Local)
			}
		}
		for {
			tok, err := fmtDec.Token()
			if err != nil {
				break
			}
			if se, ok := tok.(xml.StartElement); ok {
				fmtElements = append(fmtElements, se.Name.Local)
			}
		}
		if len(origElements) > 0 && !slices.Equal(origElements, fmtElements) {
			t.Fatalf("XML element structure changed:\n  orig: %v\n  fmt:  %v", origElements, fmtElements)
		}
	})
}
