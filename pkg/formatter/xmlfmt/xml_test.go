package xmlfmt_test

import (
	"bytes"
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
)

var update = flag.Bool("update", false, "update .expected.* golden files")

var f = xmlfmt.Formatter{}
var defaultOpts = xmlfmt.DefaultOptions()

// TestFixtures runs all .input.xml -> .expected.xml fixture pairs.
// Pass -update to regenerate golden files from current formatter output.
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

			optsFile := "testdata/" + name + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			got, err := f.Format(src, opts)
			require.NoError(t, err, "Format(%s) should not error", name)

			if *update {
				require.NoError(t, os.WriteFile(expected, got, 0o600), //nolint:gosec // path derived from glob within testdata/
					"failed to update golden file %s", expected)
				return
			}

			want, err := os.ReadFile(expected)
			require.NoError(t, err)
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

// TestTabIndent verifies that IndentStyle=IndentTabs uses tab indentation.
func TestTabIndent(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<root><child>value</child></root>\n")
	opts := xmlfmt.DefaultOptions()
	opts.IndentStyle = formatter.IndentTabs

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\t<child>")
	require.NotContains(t, string(got), "  <child>")
}

// TestSelfClosingSpaceRemoved verifies that XMLSelfClosingSpace=false removes
// the space before /> in self-closing tags that already have one.
func TestSelfClosingSpaceRemoved(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<config>\n  <server host=\"localhost\" />\n</config>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLSelfClosingSpace = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), `host="localhost"/>`)
	require.NotContains(t, string(got), `host="localhost" />`)
}

// TestSelfClosingSpaceAdded verifies that XMLSelfClosingSpace=true adds
// a space before /> in self-closing tags that don't have one.
func TestSelfClosingSpaceAdded(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<config>\n  <server host=\"localhost\"/>\n</config>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLSelfClosingSpace = true

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), `host="localhost" />`)
}

// TestMultiLineSelfCloseIdempotent verifies that multi-line self-closing tags
// with /> on its own indented line don't lose indentation on re-format (Bug 6).
func TestMultiLineSelfCloseIdempotent(t *testing.T) {
	t.Parallel()
	// Multi-line self-closing tag with /> on its own line.
	src := []byte("<?xml version=\"1.0\"?>\n<assembly>\n  <assemblyIdentity\n      type=\"win32\"\n      name=\"app\"\n      />\n</assembly>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLSelfClosingSpace = false

	first, err := f.Format(src, opts)
	require.NoError(t, err)
	// The /> should remain indented (not lose spaces each pass).
	second, err := f.Format(first, opts)
	require.NoError(t, err)
	third, err := f.Format(second, opts)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second),
		"multi-line self-close must be idempotent pass 1→2")
	require.Equal(t, string(second), string(third),
		"multi-line self-close must be idempotent pass 2→3")
}

// TestMultiSpaceSelfCloseStrippedInOnePass verifies that multiple spaces before />
// are all removed in a single pass, not one-at-a-time (Bug 8).
func TestMultiSpaceSelfCloseStrippedInOnePass(t *testing.T) {
	t.Parallel()
	// Triple space before />
	src := []byte("<?xml version=\"1.0\"?>\n<doc>\n  <e1   />\n  <e3   name=\"elem3\"   id=\"elem3\"  />\n</doc>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLSelfClosingSpace = false

	first, err := f.Format(src, opts)
	require.NoError(t, err)
	second, err := f.Format(first, opts)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second),
		"multi-space self-close must be idempotent (all spaces removed in one pass)")
	require.Contains(t, string(first), "<e1/>")
	require.Contains(t, string(first), `id="elem3"/>`)
}

// TestMultiSpaceSelfCloseCollapsedToOne verifies that multiple spaces before />
// are collapsed to exactly one when XMLSelfClosingSpace=true (Bug 8).
func TestMultiSpaceSelfCloseCollapsedToOne(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<doc>\n  <e1   />\n</doc>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLSelfClosingSpace = true

	first, err := f.Format(src, opts)
	require.NoError(t, err)
	second, err := f.Format(first, opts)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second),
		"multi-space self-close must be idempotent (collapsed to single space in one pass)")
	require.Contains(t, string(first), "<e1 />")
	require.NotContains(t, string(first), "<e1  />")
}

// TestTrailingWhitespaceStripped verifies that trailing spaces on lines are removed.
func TestTrailingWhitespaceStripped(t *testing.T) {
	t.Parallel()
	// Input with trailing spaces after element content.
	src := []byte("<?xml version=\"1.0\"?>\n<root>\n  <item>value   </item>\n</root>\n")
	opts := xmlfmt.DefaultOptions()

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	// No line should end with trailing spaces.
	for _, line := range strings.Split(string(got), "\n") {
		require.Equal(t, strings.TrimRight(line, " \t"), line,
			"line has trailing whitespace: %q", line)
	}
}

// TestCRLFInputStripsTrailingWhitespace verifies that CRLF input has trailing
// whitespace stripped even on Windows-style line endings.
func TestCRLFInputStripsTrailingWhitespace(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\r\n<root>\r\n  <item>v</item>  \r\n</root>\r\n")
	opts := xmlfmt.DefaultOptions()
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEmpty(t, got)
}

// TestZeroIndentWidthDefaults verifies that IndentWidth=0 defaults to 2 spaces.
func TestZeroIndentWidthDefaults(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<root><child>v</child></root>\n")
	opts := xmlfmt.DefaultOptions()
	opts.IndentWidth = 0

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "  <child>")
}

// TestTrailingWhitespaceStrippedCRLF verifies CRLF lines have trailing spaces stripped.
func TestTrailingWhitespaceStrippedCRLF(t *testing.T) {
	t.Parallel()
	// Input with trailing spaces on CRLF lines.
	src := []byte("<?xml version=\"1.0\"?>\r\n<root>  \r\n  <item>v</item>  \r\n</root>\r\n")
	opts := xmlfmt.DefaultOptions()
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	// No line should end with trailing spaces before CRLF.
	for _, line := range strings.Split(strings.ReplaceAll(string(got), "\r\n", "\n"), "\n") {
		require.Equal(t, strings.TrimRight(line, " \t"), line,
			"line has trailing whitespace: %q", line)
	}
}

// TestContentAfterLastNewline verifies that content not followed by a newline
// is included in the output (exercises the final lineStart < len(data) path).
func TestContentAfterLastNewline(t *testing.T) {
	t.Parallel()
	// A well-formed XML with no trailing newline.
	src := []byte("<?xml version=\"1.0\"?><root><item>v</item></root>")
	opts := xmlfmt.DefaultOptions()
	opts.FinalNewline = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.NotEqual(t, byte('\n'), got[len(got)-1])
}

// TestBOMPreserved verifies that a UTF-8 BOM at the start of the file is
// preserved through formatting.
func TestBOMPreserved(t *testing.T) {
	t.Parallel()
	// UTF-8 BOM followed by valid XML.
	bom := []byte{0xef, 0xbb, 0xbf}
	src := append(bom, []byte("<?xml version=\"1.0\"?>\n<root><child>v</child></root>\n")...)
	opts := xmlfmt.DefaultOptions()

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.True(t, len(got) >= 3 && got[0] == 0xef && got[1] == 0xbb && got[2] == 0xbf,
		"BOM must be preserved at start of output")
}

// TestPreserveModeReindentsExisting verifies that XMLWhitespacePreserve mode
// only modifies existing indent tokens rather than inserting new ones.
func TestPreserveModeReindentsExisting(t *testing.T) {
	t.Parallel()
	src := []byte("<?xml version=\"1.0\"?>\n<root>\n\t\t<child>v</child>\n</root>\n")
	opts := xmlfmt.DefaultOptions()
	opts.XMLWhitespaceSensitivity = formatter.XMLWhitespacePreserve

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	// Preserve mode normalizes indent to 2 spaces.
	require.Contains(t, string(got), "  <child>")
	require.NotContains(t, string(got), "\t\t<child>")
}

// TestTextBeforeTagPrevIsTagFalse exercises the prevIsTag=false path where
// a text node immediately precedes the current token (not a tag).
func TestTextBeforeTagPrevIsTagFalse(t *testing.T) {
	t.Parallel()
	// XML with text content followed by a closing tag on the same line —
	// exercises the default branch in prevIsTag where prev is TokText.
	src := []byte("<?xml version=\"1.0\"?><root>hello</root>")
	opts := xmlfmt.DefaultOptions()

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	// Idempotent.
	got2, err := f.Format(got, opts)
	require.NoError(t, err)
	require.Equal(t, string(got), string(got2))
}

// TestTextFollowedByElement verifies that a text node directly followed by
// a child element is formatted correctly. This exercises the TokText path
// in insertFormattingWhitespace and the prevIsTag=false path in prevIsTag.
func TestTextFollowedByElement(t *testing.T) {
	t.Parallel()
	// XML where the root has leading text before its child element.
	// helium accepts this as valid mixed content.
	src := []byte("<?xml version=\"1.0\"?><root>prefix <child>value</child></root>")
	opts := xmlfmt.DefaultOptions()

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEmpty(t, got)

	// Must be idempotent.
	got2, err := f.Format(got, opts)
	require.NoError(t, err)
	require.Equal(t, string(got), string(got2))
}

// FuzzXMLFormatter seeds from fixture files and checks idempotency.
func FuzzXMLFormatter(f *testing.F) {
	inputs, _ := filepath.Glob("testdata/*.input.xml")
	for _, path := range inputs {
		data, _ := os.ReadFile(path)
		f.Add(data)
	}

	fmter := xmlfmt.Formatter{}
	opts := xmlfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, opts)
		if err != nil {
			return
		}

		result2, err := fmter.Format(result, opts)
		if err != nil {
			t.Fatalf("second format pass failed: %v\nfirst output: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent:\ninput:  %q\nfirst:  %q\nsecond: %q", data, result, result2)
		}
	})
}
