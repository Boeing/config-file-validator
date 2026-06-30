package formatter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// TestOptionsZeroValueIsDefault verifies that zero-value Options means
// "use format defaults" — not "force everything to zero."
func TestOptionsZeroValueIsDefault(t *testing.T) {
	var opts formatter.Options
	require.Equal(t, formatter.IndentDefault, opts.IndentStyle)
	require.Equal(t, 0, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingDefault, opts.LineEnding)
	require.False(t, opts.FinalNewline)
	require.False(t, opts.SortKeys)
	require.Equal(t, 0, opts.MaxLineWidth)
}
