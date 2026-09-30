package formatter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

func TestNormalizeLineEndings_LFPassthrough(t *testing.T) {
	t.Parallel()
	input := []byte("line1\nline2\nline3\n")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingLF)
	// LF input with LF ending — returned unchanged (same backing array, no copy)
	require.Equal(t, input, got)
}

func TestNormalizeLineEndings_DefaultPassthrough(t *testing.T) {
	t.Parallel()
	input := []byte("line1\nline2\n")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingDefault)
	require.Equal(t, input, got)
}

func TestNormalizeLineEndings_CRLFConversion(t *testing.T) {
	t.Parallel()
	input := []byte("line1\nline2\nline3\n")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingCRLF)
	require.Equal(t, []byte("line1\r\nline2\r\nline3\r\n"), got)
}

func TestNormalizeLineEndings_AlreadyCRLFUnchanged(t *testing.T) {
	t.Parallel()
	// Input already has CRLF — must not become CRCRLF.
	input := []byte("line1\r\nline2\r\n")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingCRLF)
	require.Equal(t, input, got)
}

func TestNormalizeLineEndings_MixedLineEndings(t *testing.T) {
	t.Parallel()
	// Some lines have CRLF already, others have bare LF.
	input := []byte("a\r\nb\nc\r\n")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingCRLF)
	require.Equal(t, []byte("a\r\nb\r\nc\r\n"), got)
}

func TestNormalizeLineEndings_EmptyInput(t *testing.T) {
	t.Parallel()
	got := formatter.NormalizeLineEndings([]byte{}, formatter.LineEndingCRLF)
	require.Empty(t, got)
}

func TestNormalizeLineEndings_NoNewlines(t *testing.T) {
	t.Parallel()
	input := []byte("no newlines here")
	got := formatter.NormalizeLineEndings(input, formatter.LineEndingCRLF)
	require.Equal(t, input, got)
}
