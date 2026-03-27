package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ArrToMap(t *testing.T) {
	t.Parallel()

	result := ArrToMap("a", "b", "c")
	require.Len(t, result, 3)
	_, ok := result["a"]
	require.True(t, ok)
	_, ok = result["d"]
	require.False(t, ok)
}

func Test_ArrToMapEmpty(t *testing.T) {
	t.Parallel()

	result := ArrToMap()
	require.Empty(t, result)
}

func Test_ArrToMapDuplicates(t *testing.T) {
	t.Parallel()

	result := ArrToMap("a", "a", "b")
	require.Len(t, result, 2)
}
