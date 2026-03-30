package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/internal/testhelper"
)

func Test_getFlags(t *testing.T) {
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
		{"format json", []string{"--check-format=json", "."}, false},
		{"schema sarif", []string{"--schema=sarif", "."}, false},
		{"version flag", []string{"--version"}, false},
		{"exclude file types with empty element", []string{"--exclude-file-types=json,,yaml", "."}, false},
		{"type-map", []string{"--type-map=**/inventory:ini", "."}, false},
		{"multiple type-maps", []string{"--type-map=**/inventory:ini", "--type-map=**/configs/*:properties", "."}, false},

		// Invalid flag combinations
		{"negative depth", []string{"-depth=-1", "."}, true},
		{"wrong reporter", []string{"--reporter=wrong", "."}, true},
		{"bad reporter format", []string{"--reporter", "json:/a:/b", "."}, true},
		{"invalid groupby", []string{"-groupby=badgroup", "."}, true},
		{"groupby duplicate", []string{"--groupby=directory,directory", "."}, true},
		{"grouped junit", []string{"-groupby=directory", "--reporter=junit", "."}, true},
		{"grouped sarif", []string{"-groupby=directory", "--reporter=sarif", "."}, true},
		{"format all includes unsupported", []string{"--check-format=all", "."}, false},
		{"format with unsupported types", []string{"--check-format=json,yaml,ini", "."}, false},
		{"invalid format type", []string{"--check-format=notreal", "."}, true},
		{"invalid schema type", []string{"--schema=notreal", "."}, true},
		{"invalid exclude file type", []string{"--exclude-file-types=notreal", "."}, true},
		{"invalid file type", []string{"--file-types=notreal", "."}, true},
		{"file-types and exclude-file-types together", []string{"--file-types=json", "--exclude-file-types=yaml", "."}, true},
		{"help flag", []string{"--help"}, true}, // flag.ErrHelp
		{"globbing with exclude-dirs", []string{"-globbing", "--exclude-dirs=subdir", "."}, true},
		{"globbing with exclude-file-types", []string{"-globbing", "--exclude-file-types=hcl", "."}, true},
		{"globbing with file-types", []string{"-globbing", "--file-types=json", "."}, true},
		{"globbing with bad pattern", []string{"-globbing", "/nonexistent/["}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := getFlags(tc.args)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_getFlagsValues(t *testing.T) {
	cfg, err := getFlags([]string{"-depth=3", "--exclude-dirs=vendor,node_modules", "--schema=sarif", "--check-format=json", "."})
	require.NoError(t, err)

	require.Equal(t, 3, *cfg.depth)
	require.Equal(t, "vendor,node_modules", *cfg.excludeDirs)
	require.Equal(t, "sarif", *cfg.schema)
	require.Equal(t, "json", *cfg.format)
	require.Equal(t, []string{"."}, cfg.searchPaths)
}

// Integration tests that need the full mainInit pipeline with real files
func Test_mainInit(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	jsonFile := testhelper.CreateFixtureFile(t, "json")
	tomlFile := testhelper.CreateFixtureFile(t, "toml")
	sarifFile := testhelper.CreateFixtureFile(t, "sarif")
	jsonDir := testhelper.CreateFixtureDir(t, "json", "yaml")
	formattedDir := t.TempDir()
	testhelper.WriteFile(t, formattedDir, "good.json", "{\n  \"key\": \"value\"\n}\n")

	cases := []struct {
		name         string
		args         []string
		expectedExit int
	}{
		{"version", []string{"--version"}, 0},
		{"help", []string{"--help"}, 0},
		{"bad path", []string{"/path/does/not/exit"}, 1},
		{"multiple paths", []string{jsonFile, tomlFile}, 0},
		{"schema sarif", []string{"--schema=sarif", sarifFile}, 0},
		{"format json", []string{"--check-format=json", formattedDir}, 0},
		{"format all unsupported", []string{"--check-format=all", jsonFile}, 1},
		{"format unsupported types", []string{"--check-format=json,yaml,ini", jsonFile}, 1},
		{"format with multiple files", []string{"--check-format=json", formattedDir + "/good.json", tomlFile}, 0},
		{"file-types filter", []string{"--file-types=json", jsonFile}, 0},
		{"depth set", []string{"-depth=1", jsonDir}, 0},
		{"output to dir", []string{"--reporter=json:" + t.TempDir(), jsonDir}, 0},
		{"output to dir standard", []string{"--reporter=standard:" + t.TempDir(), jsonDir}, 0},
		{"output to bad path", []string{"--reporter", "json:/path/not/exist", jsonDir}, 1},
		{"junit reporter", []string{"--reporter=junit", jsonDir}, 0},
		{"sarif reporter", []string{"--reporter=sarif", jsonDir}, 0},
		{"globbing with pattern", []string{"--globbing=true", jsonDir + "/*.json"}, 0},
		{"globbing no matches", []string{"--globbing=true", jsonDir + "/*.nomatch"}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Args = append([]string{"validator"}, tc.args...)
			actual := mainInit()
			require.Equal(t, tc.expectedExit, actual)
		})
	}
}

func Test_envVarFallback(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	t.Setenv("CFV_DEPTH", "2")
	t.Setenv("CFV_QUIET", "true")

	os.Args = []string{"validator", "."}
	require.Equal(t, 0, mainInit())
}

func Test_envVarInvalid(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	t.Setenv("CFV_DEPTH", "notanumber")

	os.Args = []string{"validator", "."}
	require.Equal(t, 1, mainInit())
}

func Test_getExcludeFileTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"exclude yaml", "yaml", []string{"yaml", "yml"}},
		{"exclude yml", "yml", []string{"yaml", "yml"}},
		{"exclude json", "json", []string{"json"}},
		{"exclude json and yaml", "json,yaml", []string{"json", "yaml", "yml"}},
		{"case insensitive", "jSon,YamL", []string{"json", "yaml", "yml"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.expected, getExcludeFileTypes(tc.input))
		})
	}
}

func Test_getFormatFileTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", []string{}},
		{"json", "json", []string{"json"}},
		{"dedup", "json,json", []string{"json"}},
		{"json and yaml", "json,yaml", []string{"json", "yaml"}},
		{"all", "all,json", []string{
			"json", "yaml", "xml", "toml", "ini", "properties",
			"hcl", "plist", "csv", "hocon", "env", "editorconfig",
			"toon", "sarif",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.expected, getFormatFileTypes(tc.input))
		})
	}
}

func Test_getSchemaFileTypes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", []string{}},
		{"sarif", "sarif", []string{"sarif"}},
		{"multiple", "sarif,json", []string{"sarif", "json"}},
		{"dedup", "sarif,sarif", []string{"sarif"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.expected, getSchemaFileTypes(tc.input))
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

func Test_mainInitTypeMap(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "inventory", "[servers]\nhost=10.0.0.1\n")

	os.Args = []string{"validator", "--type-map=**/inventory:ini", dir}
	require.Equal(t, 0, mainInit())
}
