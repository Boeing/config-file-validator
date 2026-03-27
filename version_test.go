package configfilevalidator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetVersion(t *testing.T) {
	v := GetVersion()
	require.Equal(t, "unknown", v.Version)
}

func Test_VersionString(t *testing.T) {
	v := Version{Version: "1.0.0"}
	require.Equal(t, "validator version 1.0.0", v.String())
}
