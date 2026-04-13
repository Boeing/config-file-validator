package filetype

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinguistKnownFilesPopulated(t *testing.T) {
	t.Parallel()
	// Verify that the generated LinguistKnownFiles map is non-empty
	require.NotEmpty(t, LinguistKnownFiles, "LinguistKnownFiles should be populated by the generator")
}

func TestInitMergesLinguistKnownFiles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		fileType *FileType
		file     string
	}{
		{&JSONFileType, ".arcconfig"},
		{&JSONCFileType, ".babelrc"},
		{&JSONCFileType, "tsconfig.json"},
		{&YAMLFileType, ".clang-format"},
		{&YAMLFileType, ".clangd"},
		{&XMLFileType, "pom.xml"},
		{&XMLFileType, ".classpath"},
		{&TomlFileType, "Pipfile"},
		{&IniFileType, ".gitconfig"},
		{&IniFileType, ".npmrc"},
	}

	for _, tc := range cases {
		t.Run(tc.fileType.Name+"/"+tc.file, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, tc.fileType.KnownFiles, "KnownFiles should be initialized for %s", tc.fileType.Name)
			_, ok := tc.fileType.KnownFiles[tc.file]
			require.True(t, ok, "%s should be a known file for %s", tc.file, tc.fileType.Name)
		})
	}
}

func TestInitMergesExtraKnownFiles(t *testing.T) {
	t.Parallel()
	// .shellcheckrc is in extraKnownFiles but not in Linguist
	_, ok := IniFileType.KnownFiles[".shellcheckrc"]
	require.True(t, ok, ".shellcheckrc should be a known file for ini (from extras)")
}

func TestFileTypesSliceHasKnownFiles(t *testing.T) {
	t.Parallel()
	for _, ft := range FileTypes {
		if len(ft.KnownFiles) > 0 {
			require.NotNil(t, ft.KnownFiles, "FileTypes[%s].KnownFiles should be initialized", ft.Name)
		}
	}
}

func TestEditorConfigNotInINIKnownFiles(t *testing.T) {
	t.Parallel()
	_, ok := IniFileType.KnownFiles[".editorconfig"]
	require.False(t, ok, ".editorconfig should NOT be in INI KnownFiles")
}
