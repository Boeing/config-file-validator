package xmlfmt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXMLTokenizeLossless(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"simple element", "<root><child/></root>\n"},
		{"with indent", "<root>\n  <child/>\n</root>\n"},
		{"attributes", `<img src="test.png" alt="a>b"/>` + "\n"},
		{"comment", "<!-- comment -->\n<root/>\n"},
		{"multiline comment", "<!--\n  multi\n  line\n-->\n<root/>\n"},
		{"cdata", "<root><![CDATA[<not a tag>]]></root>\n"},
		{"processing instruction", "<?xml-stylesheet type=\"text/xsl\"?>\n<root/>\n"},
		{"xml declaration", `<?xml version="1.0" encoding="UTF-8"?>` + "\n<root/>\n"},
		{"doctype", "<!DOCTYPE html>\n<html/>\n"},
		{"doctype with subset", "<!DOCTYPE doc [\n  <!ELEMENT doc (#PCDATA)>\n]>\n<doc/>\n"},
		{"mixed content", "<p>Hello <b>world</b>!</p>\n"},
		{"self closing", "<br/>\n"},
		{"nested", "<a>\n  <b>\n    <c/>\n  </b>\n</a>\n"},
		{"text and elements", "<root>\n  <name>John</name>\n  <age>30</age>\n</root>\n"},
		{"crlf", "<root>\r\n  <child/>\r\n</root>\r\n"},
		{"no final newline", "<root/>"},
		{"tab indent", "<root>\n\t<child/>\n</root>\n"},
		{"close tag", "</root>\n"},
		{"entities", "<root>&lt;&gt;&amp;</root>\n"},
		{"attribute with single quotes", "<tag attr='value'/>\n"},
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
				"losslessness violated")
		})
	}
}

// TestXMLTokenKindClassification verifies that specific XML constructs are
// classified as the correct token kind.
func TestXMLTokenKindClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		src          string
		expectedKind TokenKind
		tokenIndex   int // which token to check (0-based)
	}{
		{
			name:         "xml_decl_is_TokXMLDecl",
			src:          `<?xml version="1.0"?>`,
			expectedKind: TokXMLDecl,
			tokenIndex:   0,
		},
		{
			name:         "xml_stylesheet_is_TokProcInst",
			src:          `<?xml-stylesheet type="text/xsl" href="style.xsl"?>`,
			expectedKind: TokProcInst,
			tokenIndex:   0,
		},
		{
			name:         "xml_model_is_TokProcInst",
			src:          `<?xml-model href="schema.rnc"?>`,
			expectedKind: TokProcInst,
			tokenIndex:   0,
		},
		{
			name:         "regular_pi_is_TokProcInst",
			src:          `<?php echo "hi"; ?>`,
			expectedKind: TokProcInst,
			tokenIndex:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tokens := tokenize([]byte(tc.src))
			require.Greater(t, len(tokens), tc.tokenIndex,
				"not enough tokens produced for input %q", tc.src)
			require.Equal(t, tc.expectedKind, tokens[tc.tokenIndex].Kind,
				"wrong token kind for %q", tc.src)
		})
	}
}

func FuzzXMLTokenizeLossless(f *testing.F) {
	f.Add([]byte("<root/>"))
	f.Add([]byte("<root>\n  <child/>\n</root>\n"))
	f.Add([]byte("<!-- comment -->\n"))
	f.Add([]byte("<![CDATA[data]]>"))
	f.Add([]byte(`<?xml version="1.0"?>`))
	f.Add([]byte("<!DOCTYPE html>"))
	f.Add([]byte("<p>text <b>bold</b> more</p>"))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0xFF})
	f.Add([]byte("<a attr=\">\"/>"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tokens := tokenize(data)
		var reconstructed []byte
		for _, tok := range tokens {
			reconstructed = append(reconstructed, tok.Raw...)
		}
		if string(reconstructed) != string(data) {
			t.Fatalf("losslessness violated.\nInput: %q\nGot:   %q", data, reconstructed)
		}
	})
}
