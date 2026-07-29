package xmlfmt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
)

// TestTextContentPreservation verifies that multi-line text content inside
// XML elements is preserved during formatting. This was a critical bug:
// removeInsignificantWhitespace stripped ALL newlines, causing text like
// "Line one.\nLine two." to become "Line one.Line two." (data corruption).
func TestTextContentPreservation(t *testing.T) {
	t.Parallel()
	fmter := xmlfmt.Formatter{}
	opts := xmlfmt.DefaultOptions()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single_line_text_stays_inline",
			input:    `<?xml version="1.0"?><root><name>John</name></root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <name>John</name>\n</root>\n",
		},
		{
			name: "multiline_text_preserves_newlines",
			input: `<?xml version="1.0"?>
<root>
  <desc>
    Line one.
    Line two.
  </desc>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <desc>\n    Line one.\n    Line two.\n  </desc>\n</root>\n",
		},
		{
			name: "multiline_text_reindents_to_correct_depth",
			input: `<?xml version="1.0"?>
<root>
<desc>
        Line one.
        Line two.
</desc>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <desc>\n    Line one.\n    Line two.\n  </desc>\n</root>\n",
		},
		{
			name: "deeply_nested_multiline_text",
			input: `<?xml version="1.0"?>
<root>
  <a>
    <b>
      <c>
        Deep text one.
        Deep text two.
      </c>
    </b>
  </a>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <a>\n    <b>\n      <c>\n        Deep text one.\n        Deep text two.\n      </c>\n    </b>\n  </a>\n</root>\n",
		},
		{
			name: "text_with_blank_line_preserved",
			input: `<?xml version="1.0"?>
<root>
  <desc>
    First paragraph.

    Second paragraph.
  </desc>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <desc>\n    First paragraph.\n\n    Second paragraph.\n  </desc>\n</root>\n",
		},
		{
			name: "element_only_unchanged",
			input: `<?xml version="1.0"?>
<root>
<items>
<item/>
<item/>
</items>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <items>\n    <item/>\n    <item/>\n  </items>\n</root>\n",
		},
		{
			name: "mixed_content_stays_inline",
			input: `<?xml version="1.0"?>
<root>
<p>Hello <b>world</b> and goodbye</p>
</root>`,
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <p>Hello <b>world</b> and goodbye</p>\n</root>\n",
		},
		{
			name: "text_and_element_siblings",
			input: `<?xml version="1.0"?>
<class>
  <brief_description>
    Short text.
  </brief_description>
  <members>
    <member name="x" type="int">Value</member>
  </members>
</class>`,
			expected: "<?xml version=\"1.0\"?>\n<class>\n  <brief_description>\n    Short text.\n  </brief_description>\n  <members>\n    <member name=\"x\" type=\"int\">Value</member>\n  </members>\n</class>\n",
		},
		{
			name: "godot_style_description",
			input: `<?xml version="1.0" encoding="UTF-8"?>
<class name="Node">
	<description>
		Nodes are building blocks.
		A tree of nodes is called a scene.
		Scenes can be saved to disk.
	</description>
	<methods>
		<method name="add_child">
			<description>
				Adds a child node.
				The node becomes a child.
			</description>
		</method>
	</methods>
</class>`,
			expected: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<class name=\"Node\">\n  <description>\n    Nodes are building blocks.\n    A tree of nodes is called a scene.\n    Scenes can be saved to disk.\n  </description>\n  <methods>\n    <method name=\"add_child\">\n      <description>\n        Adds a child node.\n        The node becomes a child.\n      </description>\n    </method>\n  </methods>\n</class>\n",
		},
		{
			name: "text_without_source_indent_preserved_at_col0",
			input: `<?xml version="1.0"?>
<root>
<desc>
Line one.
Line two.
</desc>
</root>`,
			// When source has no indent before text, text stays at column 0.
			// This matches xmllint behavior: source whitespace is preserved.
			expected: "<?xml version=\"1.0\"?>\n<root>\n  <desc>\nLine one.\nLine two.\n  </desc>\n</root>\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := fmter.Format([]byte(tc.input), opts)
			require.NoError(t, err)
			require.Equal(t, tc.expected, string(got),
				"formatting mismatch for %s", tc.name)

			// Verify idempotency.
			got2, err := fmter.Format(got, opts)
			require.NoError(t, err)
			require.Equal(t, string(got), string(got2),
				"not idempotent for %s", tc.name)
		})
	}
}

// TestTextContentPreserveModeStillWorks verifies that XMLWhitespacePreserve
// mode continues to work correctly — it only reindents existing indentation
// without inserting or removing any tokens.
func TestTextContentPreserveModeStillWorks(t *testing.T) {
	t.Parallel()
	fmter := xmlfmt.Formatter{}
	opts := xmlfmt.DefaultOptions()
	opts.XMLWhitespaceSensitivity = formatter.XMLWhitespacePreserve

	// Preserve mode: tabs → 2-space indent, but no structural changes.
	src := "<?xml version=\"1.0\"?>\n<root>\n\t<desc>\n\t\tLine one.\n\t\tLine two.\n\t</desc>\n</root>\n"
	got, err := fmter.Format([]byte(src), opts)
	require.NoError(t, err)

	expected := "<?xml version=\"1.0\"?>\n<root>\n  <desc>\n    Line one.\n    Line two.\n  </desc>\n</root>\n"
	require.Equal(t, expected, string(got))

	// Idempotent.
	got2, err := fmter.Format(got, opts)
	require.NoError(t, err)
	require.Equal(t, string(got), string(got2))
}

// TestTextContentWithTabIndent verifies text content works with tab indentation.
func TestTextContentWithTabIndent(t *testing.T) {
	t.Parallel()
	fmter := xmlfmt.Formatter{}
	opts := xmlfmt.DefaultOptions()
	opts.IndentStyle = formatter.IndentTabs

	src := `<?xml version="1.0"?>
<root>
  <desc>
    Line one.
    Line two.
  </desc>
</root>`

	got, err := fmter.Format([]byte(src), opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\t<desc>")
	require.Contains(t, string(got), "\t\tLine one.")
	require.Contains(t, string(got), "\t\tLine two.")
	require.Contains(t, string(got), "\t</desc>")

	// Idempotent.
	got2, err := fmter.Format(got, opts)
	require.NoError(t, err)
	require.Equal(t, string(got), string(got2))
}
