package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/stretchr/testify/require"
)

// TestMain wires the cfv binary into testscript so txtar tests can invoke it.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"cfv": main,
	})
}

// TestScript runs all .txtar integration tests in testdata/.
func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
	})
}

// --- Unit tests for flag parsing and config resolution ---

func Test_parseCheckFlags(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		// Valid flag combinations
		{"no args defaults to cwd", []string{}, false},
		{"depth set", []string{"-depth=1", "."}, false},
		{"exclude dirs", []string{"--exclude-dirs=subdir", "."}, false},
		{"exclude file types", []string{"--exclude-file-types=json,yaml", "."}, false},
		{"file types", []string{"--file-types=json", "."}, false},
		{"quiet flag", []string{"--quiet=true", "."}, false},
		{"json reporter", []string{"--reporter=json", "."}, false},
		{"junit reporter", []string{"--reporter=junit", "."}, false},
		{"sarif reporter", []string{"--reporter=sarif", "."}, false},
		{"json and junit reporter", []string{"--reporter=json:-", "--reporter=junit:-", "."}, false},
		{"groupby directory", []string{"-groupby=directory", "."}, false},
		{"exclude file types with empty element", []string{"--exclude-file-types=json,,yaml", "."}, false},
		{"type-map", []string{"--type-map=**/inventory:ini", "."}, false},
		{"multiple type-maps", []string{"--type-map=**/inventory:ini", "--type-map=**/configs/*:properties", "."}, false},
		{"require-schema", []string{"--require-schema", "."}, false},
		{"sarif merge file", []string{"--reporter=sarif", "--merge-sarif=external.sarif", "."}, false},
		{"sarif merge dir", []string{"--reporter=sarif", "--merge-sarif-dir=reports", "."}, false},
		{"ignore-file", []string{"--ignore-file=.dockerignore", "."}, false},
		{"multiple ignore-files", []string{"--ignore-file=.dockerignore", "--ignore-file=.prettierignore", "."}, false},
		{"fix flag reserved no-op", []string{"--fix", "."}, false},
		{"unsafe flag reserved no-op", []string{"--fix", "--unsafe", "."}, false},

		// Invalid flag combinations
		{"negative depth", []string{"-depth=-1", "."}, true},
		{"wrong reporter", []string{"--reporter=wrong", "."}, true},
		{"merge sarif requires sarif reporter", []string{"--reporter=json", "--merge-sarif=external.sarif", "."}, true},
		{"empty merge sarif file", []string{"--reporter=sarif", "--merge-sarif=", "."}, true},
		{"empty merge sarif file without sarif reporter", []string{"--reporter=json", "--merge-sarif=", "."}, true},
		{"empty merge sarif dir", []string{"--reporter=sarif", "--merge-sarif-dir=", "."}, true},
		{"empty merge sarif dir without sarif reporter", []string{"--reporter=json", "--merge-sarif-dir=", "."}, true},
		{"reporter output path with colon", []string{"--reporter", "json:/a:/b", "."}, false},
		{"invalid groupby", []string{"-groupby=badgroup", "."}, true},
		{"groupby duplicate", []string{"--groupby=directory,directory", "."}, true},
		{"grouped junit", []string{"-groupby=directory", "--reporter=junit", "."}, true},
		{"grouped sarif", []string{"-groupby=directory", "--reporter=sarif", "."}, true},
		{"invalid exclude file type", []string{"--exclude-file-types=notreal", "."}, true},
		{"invalid file type", []string{"--file-types=notreal", "."}, true},
		{"file-types and exclude-file-types together", []string{"--file-types=json", "--exclude-file-types=yaml", "."}, true},
		{"help flag", []string{"--help"}, true},
		{"globbing with exclude-dirs", []string{"-globbing", "--exclude-dirs=subdir", "."}, true},
		{"globbing with exclude-file-types", []string{"-globbing", "--exclude-file-types=hcl", "."}, true},
		{"globbing with file-types", []string{"-globbing", "--file-types=json", "."}, true},
		{"globbing with bad pattern", []string{"-globbing", "/nonexistent/["}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCheckFlags(tc.args)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_resolveConfigAllowsNilMergeSarifDir(t *testing.T) {
	fs := flag.NewFlagSet("cfv check", flag.ContinueOnError)

	empty := ""
	depth := 0
	falseVal := false
	trueVal := true
	cfg := &cfvConfig{
		fs:               fs,
		searchPaths:      []string{"."},
		excludeDirs:      &empty,
		excludeFileTypes: &empty,
		fileTypes:        &empty,
		reportType:       []reporterConfig{{reportType: "sarif"}},
		depth:            &depth,
		groupOutput:      &empty,
		quiet:            &falseVal,
		globbing:         &falseVal,
		requireSchema:    &falseVal,
		noSchema:         &falseVal,
		schemaStore:      &falseVal,
		schemaStorePath:  &empty,
		configPath:       &empty,
		noConfig:         &trueVal,
		gitignore:        &falseVal,
		mergeSarifDir:    nil,
		fix:              &falseVal,
		unsafe:           &falseVal,
	}

	require.NotPanics(t, func() {
		_, err := resolveConfig(cfg)
		require.NoError(t, err)
	})
}

func Test_parseCheckFlagsValues(t *testing.T) {
	cfg, err := parseCheckFlags([]string{"-depth=3", "--exclude-dirs=vendor,node_modules", "--require-schema", "."})
	require.NoError(t, err)

	require.Equal(t, 3, *cfg.depth)
	require.Equal(t, "vendor,node_modules", *cfg.excludeDirs)
	require.True(t, *cfg.requireSchema)
	require.Equal(t, []string{"."}, cfg.searchPaths)
}

func Test_parseCheckFlagsRejectsDuplicateReporterOutputDest(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		outputDest string
	}{
		{
			name:       "same output path",
			args:       []string{"--reporter=json:same.json", "--reporter=junit:same.json", "."},
			outputDest: "same.json",
		},
		{
			name:       "cleaned output path",
			args:       []string{"--reporter=json:./same.json", "--reporter=junit:same.json", "."},
			outputDest: "same.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCheckFlags(tc.args)
			require.ErrorContains(t, err, "multiple reporters target the same output file: "+tc.outputDest)
		})
	}
}

func Test_ignoreFilesEnvVar(t *testing.T) {
	t.Setenv("CFV_IGNORE_FILES", ".dockerignore, .prettierignore")

	cfg, err := parseCheckFlags([]string{"."})
	require.NoError(t, err)
	require.Equal(t, ignoreFileFlags{".dockerignore", ".prettierignore"}, cfg.ignoreFiles)
}

func Test_ignoreFilesConfigOverridesEnvVar(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".cfv.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`ignore-files = ["config.ignore"]`), 0600))
	t.Setenv("CFV_IGNORE_FILES", "env.ignore")

	cfg, err := parseCheckFlags([]string{"--config=" + configPath, "."})
	require.NoError(t, err)

	_, err = applyConfigFile(&cfg)
	require.NoError(t, err)
	require.Equal(t, ignoreFileFlags{"config.ignore"}, cfg.ignoreFiles)
}

func Test_ignoreFilesFlagOverridesConfigAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".cfv.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`ignore-files = ["config.ignore"]`), 0600))
	t.Setenv("CFV_IGNORE_FILES", "env.ignore")

	cfg, err := parseCheckFlags([]string{"--config=" + configPath, "--ignore-file=cli.ignore", "."})
	require.NoError(t, err)

	_, err = applyConfigFile(&cfg)
	require.NoError(t, err)
	require.Equal(t, ignoreFileFlags{"cli.ignore"}, cfg.ignoreFiles)
}

func Test_getExcludeFileTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"exclude yaml", "yaml", []string{"yaml", "yml"}},
		{"exclude yml", "yml", []string{"yaml", "yml"}},
		{"exclude json", "json", []string{"json", "jsonc"}},
		{"exclude jsonc", "jsonc", []string{"json", "jsonc"}},
		{"exclude json and yaml", "json,yaml", []string{"json", "jsonc", "yaml", "yml"}},
		{"case insensitive", "jSon,YamL", []string{"json", "jsonc", "yaml", "yml"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.expected, getExcludeFileTypes(tc.input))
		})
	}
}

func Test_parseTypeMapFlags(t *testing.T) {
	cases := []struct {
		name    string
		flags   typeMapFlags
		wantLen int
		wantErr bool
	}{
		{"empty", typeMapFlags{}, 0, false},
		{"single override", typeMapFlags{"**/inventory:ini"}, 1, false},
		{"multiple overrides", typeMapFlags{"**/inventory:ini", "**/configs/*:properties"}, 2, false},
		{"case insensitive type", typeMapFlags{"**/file:JSON"}, 1, false},
		{"missing colon", typeMapFlags{"nocolon"}, 0, true},
		{"empty pattern", typeMapFlags{":ini"}, 0, true},
		{"empty type", typeMapFlags{"pattern:"}, 0, true},
		{"unknown type", typeMapFlags{"pattern:notreal"}, 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseTypeMapFlags(tc.flags)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, result, tc.wantLen)
			}
		})
	}
}

func Test_emptyBoolEnvVarNoParseError(t *testing.T) {
	for _, envVar := range []string{"CFV_GITIGNORE", "CFV_QUIET", "CFV_GLOBBING", "CFV_REQUIRE_SCHEMA", "CFV_NO_SCHEMA", "CFV_SCHEMASTORE"} {
		t.Run(envVar, func(t *testing.T) {
			t.Setenv(envVar, "")
			_, err := parseCheckFlags([]string{"."})
			require.NoError(t, err)
		})
	}
}

func Test_subcommandRouter(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"version subcommand", []string{"version"}, 0},
		{"--version flag", []string{"--version"}, 0},
		{"help subcommand", []string{"help"}, 0},
		{"help check subcommand", []string{"help", "check"}, 0},
		{"help format subcommand", []string{"help", "format"}, 0},
		{"format stub returns 2", []string{"format", "."}, 2},
		{"format help returns 0", []string{"format", "--help"}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Save and restore os.Args
			orig := os.Args
			os.Args = append([]string{"cfv"}, tc.args...)
			t.Cleanup(func() { os.Args = orig })

			code := mainInit()
			require.Equal(t, tc.wantCode, code)
		})
	}
}
