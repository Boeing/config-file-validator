package main

import (
	"testing"

	"github.com/stretchr/testify/require"
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
		{"version flag", []string{"--version"}, false},
		{"exclude file types with empty element", []string{"--exclude-file-types=json,,yaml", "."}, false},
		{"type-map", []string{"--type-map=**/inventory:ini", "."}, false},
		{"multiple type-maps", []string{"--type-map=**/inventory:ini", "--type-map=**/configs/*:properties", "."}, false},
		{"require-schema", []string{"--require-schema", "."}, false},

		// Invalid flag combinations
		{"negative depth", []string{"-depth=-1", "."}, true},
		{"wrong reporter", []string{"--reporter=wrong", "."}, true},
		{"bad reporter format", []string{"--reporter", "json:/a:/b", "."}, true},
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
	cfg, err := getFlags([]string{"-depth=3", "--exclude-dirs=vendor,node_modules", "--require-schema", "."})
	require.NoError(t, err)

	require.Equal(t, 3, *cfg.depth)
	require.Equal(t, "vendor,node_modules", *cfg.excludeDirs)
	require.True(t, *cfg.requireSchema)
	require.Equal(t, []string{"."}, cfg.searchPaths)
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
