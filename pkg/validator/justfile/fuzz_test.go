package gojust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzParse feeds random bytes to Parse and asserts it never panics.
// It either returns a valid AST or an error — never crashes.
func FuzzParse(f *testing.F) {
	// Seed with all testdata files
	entries, err := os.ReadDir("testdata")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".just") {
				continue
			}
			data, err := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err == nil {
				f.Add(data)
			}
		}
	}

	// Seed with minimal valid inputs
	f.Add([]byte(""))
	f.Add([]byte("build:\n    echo done\n"))
	f.Add([]byte("v := \"hello\"\n"))
	f.Add([]byte("set dotenv-load\n"))
	f.Add([]byte("alias b := build\nbuild:\n    echo done\n"))
	f.Add([]byte("import 'foo.just'\n"))
	f.Add([]byte("mod bar\n"))
	f.Add([]byte("export FOO := \"bar\"\n"))
	f.Add([]byte("eager val := `cmd`\n"))
	f.Add([]byte("[private]\nbuild:\n    echo done\n"))
	f.Add([]byte("build target='all':\n    echo {{target}}\n"))
	f.Add([]byte("f(x) := x + \"!\"\n"))
	f.Add([]byte("v := if \"a\" == \"b\" { \"y\" } else { \"n\" }\n"))
	f.Add([]byte("v := \"a\" + \"b\" / \"c\"\n"))
	f.Add([]byte("v := / \"absolute\"\n"))
	f.Add([]byte("build:\n    echo '{{{{literal}}'\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		jf, err := Parse(data)
		if err != nil {
			return // parse errors are fine
		}
		// If it parsed, validate shouldn't panic
		_ = jf.Validate()
	})
}

// FuzzLexer feeds random bytes directly to the lexer.
func FuzzLexer(f *testing.F) {
	f.Add([]byte("build:\n    echo done\n"))
	f.Add([]byte("v := \"hello\\nworld\"\n"))
	f.Add([]byte("v := '\\u{1F600}'\n"))
	f.Add([]byte("build:\n    echo {{\"a\" + \"b\"}}\n"))
	f.Add([]byte("build:\n    echo '{{{{literal}}'\n"))
	f.Add([]byte("#!/usr/bin/env just\n"))
	f.Add([]byte("set shell := ['bash', '-cu']\n"))
	f.Add([]byte("[private, no-cd]\nbuild:\n    @-echo done\n"))

	f.Fuzz(func(_ *testing.T, data []byte) {
		l := newLexer(data)
		_, _ = l.lex() // must not panic
	})
}
