package reporter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_outputBytesToFile(t *testing.T) {
	golden, err := os.ReadFile("../../test/output/example/writer_example.txt")
	require.NoError(t, err)

	content := []byte("this is an example file.\nthis is for outputBytesToFile function.\n")

	t.Run("existing dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := outputBytesToFile(tmpDir, "default", "txt", content)
		require.NoError(t, err)

		actual, err := os.ReadFile(tmpDir + "/default.txt")
		require.NoError(t, err)
		assert.Equal(t, golden, actual)
	})

	t.Run("file name provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		outPath := tmpDir + "/validator_result.json"
		err := outputBytesToFile(outPath, "default", "json", content)
		require.NoError(t, err)

		actual, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Equal(t, golden, actual)
	})

	t.Run("existing dir without extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := outputBytesToFile(tmpDir, "default", "", content)
		require.NoError(t, err)

		actual, err := os.ReadFile(tmpDir + "/default")
		require.NoError(t, err)
		assert.Equal(t, golden, actual)
	})

	t.Run("empty string outputDest", func(t *testing.T) {
		err := outputBytesToFile("", "default", ".txt", content)
		require.Error(t, err)
		assert.Regexp(t, "outputDest is an empty string", err.Error())
	})

	t.Run("non-existing dir", func(t *testing.T) {
		err := outputBytesToFile("/nonexistent/path/output", "result", "", content)
		require.Error(t, err)
		assert.Regexp(t, "failed to create a file", err.Error())
	})
}
