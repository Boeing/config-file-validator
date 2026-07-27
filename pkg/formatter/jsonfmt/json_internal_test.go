package jsonfmt

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
)

func TestRestoreBlankLinesIgnoresMismatchedStructures(t *testing.T) {
	t.Parallel()

	object, err := hujson.Parse([]byte("{\"key\": \"value\"}"))
	require.NoError(t, err)
	array, err := hujson.Parse([]byte("[\"value\"]"))
	require.NoError(t, err)

	require.NotPanics(t, func() {
		restoreBlankLines(&object, &array, "  ", 0)
	})
	require.NotPanics(t, func() {
		restoreBlankLines(&array, &object, "  ", 0)
	})
}
