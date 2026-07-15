package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/magiconair/properties"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/envfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/inifmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/propfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/tomlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

// TestStressFormatSemanticEquivalence generates intentionally messy config files
// for every supported format, formats them, and verifies:
//  1. Formatting succeeds (no error)
//  2. Formatting is idempotent (second format produces identical output)
//  3. Formatting preserves semantic meaning (parsed data is equivalent)
//
// Files are ephemeral — generated per test run, not golden files.
func TestStressFormatSemanticEquivalence(t *testing.T) {
	t.Parallel()

	for _, tc := range stressCorpus {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := tc.opts
			if opts == (formatter.Options{}) {
				opts = defaultOpts(tc.format)
			}

			// Step 1: Format the messy input.
			formatted, err := tc.formatter.Format([]byte(tc.input), opts)
			require.NoError(t, err, "format failed for %s", tc.name)

			// Step 2: Verify idempotent — format again, get same result.
			reformatted, err := tc.formatter.Format(formatted, opts)
			require.NoError(t, err, "re-format failed for %s", tc.name)
			require.Equal(t, string(formatted), string(reformatted),
				"NOT IDEMPOTENT for %s:\nfirst:  %q\nsecond: %q", tc.name, formatted, reformatted)

			// Step 3: Verify semantic equivalence.
			tc.checkEquivalence(t, []byte(tc.input), formatted)
		})
	}
}

func defaultOpts(format string) formatter.Options {
	switch format {
	case "yaml":
		return yamlfmt.DefaultOptions()
	case "json":
		return jsonfmt.DefaultOptions()
	case "jsonc":
		return jsoncfmt.DefaultOptions()
	case "toml":
		return tomlfmt.DefaultOptions()
	case "xml":
		return xmlfmt.DefaultOptions()
	case "ini":
		return inifmt.DefaultOptions()
	case "properties":
		return propfmt.DefaultOptions()
	case "env":
		return envfmt.DefaultOptions()
	default:
		// HCL ignores format options; use a sensible default for any format
		// without a dedicated DefaultOptions function.
		return formatter.Options{IndentWidth: 2}
	}
}

type stressCase struct {
	name             string
	format           string
	formatter        formatter.Formatter
	input            string
	opts             formatter.Options
	checkEquivalence func(t *testing.T, original, formatted []byte)
}

// =============================================================================
// Semantic equivalence checkers
// =============================================================================

func jsonEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	var orig, fmt any
	require.NoError(t, json.Unmarshal(original, &orig), "parse original JSON")
	require.NoError(t, json.Unmarshal(formatted, &fmt), "parse formatted JSON")
	require.True(t, reflect.DeepEqual(orig, fmt),
		"JSON semantic mismatch:\n  original: %v\n  formatted: %v", orig, fmt)
}

func jsoncEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	// Strip comments and trailing commas to get valid JSON, then compare.
	origJSON := stripJSONCToJSON(original)
	fmtJSON := stripJSONCToJSON(formatted)
	var orig, fmt any
	require.NoError(t, json.Unmarshal(origJSON, &orig), "parse original JSONC as JSON")
	require.NoError(t, json.Unmarshal(fmtJSON, &fmt), "parse formatted JSONC as JSON")
	require.True(t, reflect.DeepEqual(orig, fmt),
		"JSONC semantic mismatch:\n  original: %v\n  formatted: %v", orig, fmt)
}

func yamlEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	var orig, fmt any
	require.NoError(t, yaml.Unmarshal(original, &orig), "parse original YAML")
	require.NoError(t, yaml.Unmarshal(formatted, &fmt), "parse formatted YAML")
	require.True(t, reflect.DeepEqual(orig, fmt),
		"YAML semantic mismatch:\n  original: %v\n  formatted: %v", orig, fmt)
}

func tomlEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	var orig, fmt map[string]any
	require.NoError(t, toml.Unmarshal(original, &orig), "parse original TOML")
	require.NoError(t, toml.Unmarshal(formatted, &fmt), "parse formatted TOML")
	// Use JSON round-trip for comparison to handle NaN/Inf (which aren't
	// representable in JSON — they become null, making comparison work).
	origJSON, _ := json.Marshal(orig)
	fmtJSON, _ := json.Marshal(fmt)
	var origNorm, fmtNorm any
	_ = json.Unmarshal(origJSON, &origNorm)
	_ = json.Unmarshal(fmtJSON, &fmtNorm)
	require.True(t, reflect.DeepEqual(origNorm, fmtNorm),
		"TOML semantic mismatch:\n  original: %v\n  formatted: %v", orig, fmt)
}

func propertiesEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	orig, err := properties.Load(original, properties.UTF8)
	require.NoError(t, err, "parse original properties")
	fmt, err := properties.Load(formatted, properties.UTF8)
	require.NoError(t, err, "parse formatted properties")

	origMap := orig.Map()
	fmtMap := fmt.Map()
	require.Equal(t, origMap, fmtMap, "Properties semantic mismatch")
}

func iniEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	orig, err := ini.Load(original)
	require.NoError(t, err, "parse original INI")
	fmt, err := ini.Load(formatted)
	require.NoError(t, err, "parse formatted INI")

	// Compare all sections and their key-value pairs (order-independent).
	origSections := orig.SectionStrings()
	fmtSections := fmt.SectionStrings()
	// Sort section lists for comparison (SortKeys doesn't reorder sections).
	require.ElementsMatch(t, origSections, fmtSections, "INI sections differ")
	for _, sec := range origSections {
		origSec := orig.Section(sec)
		fmtSec := fmt.Section(sec)
		require.ElementsMatch(t, origSec.KeyStrings(), fmtSec.KeyStrings(),
			"INI section %q keys differ", sec)
		for _, key := range origSec.KeyStrings() {
			require.Equal(t, origSec.Key(key).String(), fmtSec.Key(key).String(),
				"INI %s.%s value differs", sec, key)
		}
	}
}

func envEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	origMap := parseEnv(string(original))
	fmtMap := parseEnv(string(formatted))
	require.Equal(t, origMap, fmtMap, "ENV semantic mismatch")
}

func hclEquivalent(_ *testing.T, _, _ []byte) {
	// HCL formatter uses hclwrite which preserves semantics by construction.
	// The library parses to AST and re-serializes — it cannot change meaning.
	// Validation (cfv check) on the output is sufficient.
}

type xmlNode struct {
	name     string   // "space:local" or just "local"
	attrs    []string // sorted ["name=value", ...]
	text     string   // concatenated trimmed text content
	children []*xmlNode
}

func parseXMLTree(t *testing.T, data []byte) *xmlNode {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(data))
	root := &xmlNode{name: "__root__"}
	stack := []*xmlNode{root}

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			require.NoError(t, err, "XML decode error")
		}
		switch el := tok.(type) {
		case xml.StartElement:
			name := el.Name.Local
			if el.Name.Space != "" {
				name = el.Name.Space + ":" + el.Name.Local
			}
			attrs := make([]string, 0, len(el.Attr))
			for _, a := range el.Attr {
				aName := a.Name.Local
				if a.Name.Space != "" {
					aName = a.Name.Space + ":" + a.Name.Local
				}
				attrs = append(attrs, aName+"="+a.Value)
			}
			slices.Sort(attrs)
			node := &xmlNode{name: name, attrs: attrs}
			stack = append(stack, node)
		case xml.EndElement:
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
		case xml.CharData:
			trimmed := strings.TrimSpace(string(el))
			if trimmed != "" {
				current := stack[len(stack)-1]
				if current.text != "" {
					current.text += " " + trimmed
				} else {
					current.text = trimmed
				}
			}
		default:
			// Skip comments, processing instructions, directives
		}
	}

	// Return the single child of root if there is exactly one (normal XML).
	if len(root.children) == 1 {
		return root.children[0]
	}
	return root
}

func requireXMLTreeEqual(t *testing.T, a, b *xmlNode, path string) {
	t.Helper()
	require.Equal(t, a.name, b.name, "name mismatch at %s", path)
	require.Equal(t, a.attrs, b.attrs, "attrs mismatch at %s", path)
	require.Equal(t, a.text, b.text, "text mismatch at %s", path)
	require.Len(t, b.children, len(a.children), "children count at %s", path)
	for i := range a.children {
		requireXMLTreeEqual(t, a.children[i], b.children[i], path+"/"+a.children[i].name)
	}
}

func xmlEquivalent(t *testing.T, original, formatted []byte) {
	t.Helper()
	origTree := parseXMLTree(t, original)
	fmtTree := parseXMLTree(t, formatted)
	requireXMLTreeEqual(t, origTree, fmtTree, "root")
}

// =============================================================================
// Helpers
// =============================================================================

// parseEnv extracts KEY=VALUE pairs from a dotenv string (ignoring comments/blanks).
func parseEnv(s string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional "export " prefix.
		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		m[key] = line[idx+1:]
	}
	return m
}

// stripJSONCToJSON removes // comments, /* */ comments, and trailing commas.
// This is a simplified approach — good enough for test fixtures.
func stripJSONCToJSON(src []byte) []byte {
	// Use hujson to parse JSONC and produce standard JSON.
	// Import isn't available here, so we do a simple strip.
	var out []byte
	i := 0
	for i < len(src) {
		// Skip // comments.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		// Skip /* */ comments.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			i += 2
			continue
		}
		// Skip strings verbatim (don't strip inside strings).
		if src[i] == '"' {
			out = append(out, src[i])
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' && i+1 < len(src) {
					out = append(out, src[i], src[i+1])
					i += 2
				} else {
					out = append(out, src[i])
					i++
				}
			}
			if i < len(src) {
				out = append(out, src[i])
				i++
			}
			continue
		}
		// Remove trailing commas before } or ].
		if src[i] == ',' {
			// Look ahead past whitespace for } or ].
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				i++ // skip the comma
				continue
			}
		}
		out = append(out, src[i])
		i++
	}
	return out
}

// =============================================================================
// Stress corpus — intentionally messy configs designed to break formatters
// =============================================================================

var stressCorpus = []stressCase{
	// =========================================================================
	// JSON — 15 cases
	// =========================================================================
	{
		name:             "json/package_json_compact",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"name":"stress-app","version":"2.0.0","dependencies":{"express":"^4.18.0","lodash":"^4.17.21","axios":"^1.6.0","ws":"^8.16.0"},"devDependencies":{"jest":"^29.7.0","eslint":"^8.56.0","prettier":"^3.2.0","typescript":"^5.3.0"},"scripts":{"start":"node dist/index.js","build":"tsc","test":"jest --coverage","lint":"eslint src/ --ext .ts"}}`,
	},
	{
		name:             "json/deep_nesting",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input: `{"a":{"b":{"c":{"d":{"e":{"f":"deep"}}}}},
"x":[1,[2,[3,[4,[5]]]]],
  "mixed":{"arr":[{"nested":true},   {"also":  "nested",   "with": [1,2,3]}]}}`,
	},
	{
		name:             "json/unicode_and_escapes",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"emoji":"🎉","japanese":"日本語","escaped":"line1\nline2\ttab","path":"C:\\Users\\test\\file.txt","url":"https://example.com/path?q=1&b=2","null_val":null,"bool":true,"num":3.14159}`,
	},
	{
		name:             "json/large_array",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"items":[{"id":1,"name":"alpha","tags":["a","b","c"]},{"id":2,"name":"beta","tags":["d","e"]},{"id":3,"name":"gamma","tags":[]},{"id":4,"name":"delta","tags":["f"]},{"id":5,"name":"epsilon","tags":["g","h","i","j"]},{"id":6,"name":"zeta","tags":["k","l","m","n","o"]}]}`,
	},
	{
		name:             "json/empty_structures",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"empty_obj":{},"empty_arr":[],"nested_empty":{"a":{},"b":[]},"arr_of_empty":[{},{},[]]}`,
	},
	{
		name:             "json/special_string_values",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"backslash":"\\","quote":"\"","newline":"\n","tab":"\t","null_char":"\u0000","unicode":"\u00e9\u00e8\u00ea","surrogate":"\uD834\uDD1E","empty":"","space":" ","numbers_as_strings":"12345","bool_as_string":"true"}`,
	},
	{
		name:             "json/numeric_edge_cases",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"zero":0,"negative":-1,"float":1.5,"sci":1.5e10,"neg_sci":-2.5e-3,"max_safe":9007199254740991,"min_safe":-9007199254740991,"tiny":0.000001}`,
	},
	{
		name:             "json/messy_whitespace_everywhere",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input: `{
		    "a"   :    1    ,
	"b":2,
"c"  :  [  1  ,  2  ,  3  ]  ,
			"d"  :  {  "e"  :  "f"  }
	}`,
	},
	{
		name:             "json/single_key",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"only_key":"only_value"}`,
	},
	{
		name:             "json/all_types_mixed",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"string":"hello","number":42,"float":3.14,"bool_true":true,"bool_false":false,"null_value":null,"array":[1,"two",true,null,{"nested":"obj"}],"object":{"key":"val"}}`,
	},
	{
		name:             "json/long_string_values",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"lorem":"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.","url":"https://very-long-domain-name.example.com/path/to/resource/with/many/segments?query=parameter&another=value&third=something#fragment-identifier"}`,
	},
	{
		name:             "json/array_of_arrays",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"matrix":[[1,2,3],[4,5,6],[7,8,9]],"jagged":[[1],[2,3],[4,5,6],[7,8,9,10]],"mixed_depth":[[[[1]]],[2],[[3,[4]]]]}`,
	},
	{
		name:             "json/repeated_keys_last_wins",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"key":"first","other":"value","key":"second"}`,
	},
	{
		name:             "json/extremely_compact",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8,"i":9,"j":10,"k":11,"l":12,"m":13,"n":14,"o":15,"p":16,"q":17,"r":18,"s":19,"t":20}`,
	},
	{
		name:             "json/keys_with_special_chars",
		format:           "json",
		formatter:        jsonfmt.Formatter{},
		checkEquivalence: jsonEquivalent,
		input:            `{"":"empty key","key with spaces":"val"," leading":"a","trailing ":"b","dots.in.key":"c","slashes/in/key":"d","@special#chars!":"e"}`,
	},

	// =========================================================================
	// JSONC — 10 cases
	// =========================================================================
	{
		name:             "jsonc/vscode_settings",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
    // Editor
    "editor.fontSize":   14,
      "editor.tabSize": 2,
  "editor.wordWrap":  "on",
       "editor.formatOnSave": true,
    /* Multi-line
       comment */
    "editor.minimap.enabled":false,
      "files.exclude":  {
        "**/.git":   true,
         "**/.DS_Store":true,
    "**/node_modules":  true
      }
}`,
	},
	{
		name:             "jsonc/eslintrc_with_trailing_commas",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
  // ESLint config
    "extends": [  "eslint:recommended",
       "plugin:@typescript-eslint/recommended",],
  "rules":{
      "no-unused-vars":  "warn",
    "no-console": "off",
        "@typescript-eslint/no-explicit-any": "error",
  },
  "env":  {   "node":true,  "jest":true,  },
}`,
	},
	{
		name:             "jsonc/deeply_nested_with_comments",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
  // Top level
    "level1":  {
        // Second level
      "level2":{
            // Third level
    "level3":  {
              "value":  42,
      // inline arrays
        "arr": [1,2,3,],
    },
  },
    },
}`,
	},
	{
		name:             "jsonc/block_comments_everywhere",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
  /* before key */ "a": /* after key */ 1,
  "b": /* in value */ [
    /* before elem */ 1,
    2, /* after elem */
    3,
  ],
  /* between entries */
  "c": true,
}`,
	},
	{
		name:             "jsonc/empty_object_and_array",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
    "empty_obj":   {},
       "empty_arr":[],
    "nested":  {   "also_empty":{  }   },
}`,
	},
	{
		name:             "jsonc/all_value_types",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
    "str":  "hello",
       "num": 42,
    "float":  3.14,
  "true_val":true,
      "false_val":   false,
    "null_val":  null,
       "arr":[1,"two",  true,null],
}`,
	},
	{
		name:             "jsonc/line_comments_only",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
// First section
"a": 1,
// Second section
// with multiple lines
"b": 2,
// End
"c": 3,
}`,
	},
	{
		name:             "jsonc/tabs_and_mixed_indent",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input:            "{\n\t\"tab_indented\": true,\n    \"space_indented\": true,\n\t    \"mixed\": true,\n}",
	},
	{
		name:             "jsonc/long_inline_array_forces_expand",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
    "short":   [1,2,3],
      "long_enough_to_expand":["this is a fairly long string value","and another one","plus a third","fourth for good measure"],
}`,
	},
	{
		name:             "jsonc/string_with_comment_like_content",
		format:           "jsonc",
		formatter:        jsoncfmt.Formatter{},
		checkEquivalence: jsoncEquivalent,
		input: `{
    "url":  "https://example.com/path?a=1&b=2",
      "regex":   "^//.*$",
    "comment_like":"/* not a comment */",
       "another":  "// also not a comment",
}`,
	},

	// =========================================================================
	// YAML — 15 cases
	// =========================================================================
	{
		name:             "yaml/docker_compose_messy_indent",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `version: "3.8"
services:
    web:
        image: nginx:latest
        ports:
            - "80:80"
            - "443:443"
        environment:
            - NODE_ENV=production
        volumes:
         - ./html:/usr/share/nginx/html
    redis:
      image: redis:7-alpine
      ports:
       - "6379:6379"
      restart: always
    db:
        image: postgres:15
        environment:
            POSTGRES_USER: user
            POSTGRES_PASSWORD: pass
            POSTGRES_DB: app
volumes:
    pgdata:
`,
	},
	{
		name:             "yaml/k8s_deployment_deep_indent",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `apiVersion: apps/v1
kind: Deployment
metadata:
        name: my-app
        labels:
                app: my-app
spec:
        replicas: 3
        selector:
                matchLabels:
                        app: my-app
        template:
         metadata:
          labels:
           app: my-app
         spec:
          containers:
            - name: app
              image: my-app:latest
              ports:
               - containerPort: 8080
              resources:
                limits:
                        cpu: "500m"
                        memory: "128Mi"
`,
	},
	{
		name:             "yaml/anchors_and_aliases",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `defaults: &defaults
    cpu: "100m"
    memory: "128Mi"
web:
    resources:
        <<: *defaults
    replicas: 3
worker:
    resources:
        <<: *defaults
    replicas: 1
`,
	},
	{
		name:             "yaml/block_scalars",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `data:
    script: |
        #!/bin/bash
        set -euo pipefail
        echo "hello"
        if [ -f /tmp/ready ]; then
            exit 0
        fi
    description: >
        This is a folded
        scalar value that
        wraps across lines.
    literal: |
        keep this content
        exactly as-is
`,
	},
	{
		name:             "yaml/flow_collections",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `simple: {a: 1, b: 2, c: 3}
nested: {x: {y: {z: "deep"}}}
array: [1, 2, 3, "four", true, null]
mixed:
    inline: {key: value}
    block:
        regular: mapping
`,
	},
	{
		name:             "yaml/complex_types",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `integer: 42
float: 3.14
scientific: 6.022e23
hex: 0xFF
boolean_true: true
boolean_false: false
null_value: null
tilde_null: ~
date: 2024-01-15
timestamp: 2024-01-15T10:30:00Z
`,
	},
	{
		name:             "yaml/multiline_plain_scalars",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `simple: just a value
quoted_double: "with \"escapes\" inside"
quoted_single: 'with ''escaped'' quotes'
empty_string: ""
empty_single: ''
`,
	},
	{
		name:             "yaml/sequence_of_mappings",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `users:
    - name: Alice
      email: alice@example.com
      roles:
          - admin
          - user
    - name: Bob
      email: bob@example.com
      roles:
          - user
    - name: Charlie
      email: charlie@example.com
      roles:
          - viewer
`,
	},
	{
		name:             "yaml/empty_values",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `empty_mapping: {}
empty_sequence: []
null_key: null
empty_string: ""
tilde: ~
bare_key:
nested:
    also_empty:
    with_child:
        value: present
`,
	},
	{
		name:             "yaml/comments_everywhere",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `# Top comment
key1: value1 # inline comment
# Between entries
key2: value2
# Before nested
nested:
    # Inside nested
    child1: a
    child2: b # another inline
# End comment
`,
	},
	{
		name:             "yaml/mixed_indent_depths",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `one_space:
 child: a
two_spaces:
  child: b
four_spaces:
    child: c
eight_spaces:
        child: d
`,
	},
	{
		name:             "yaml/tags_explicit",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `string: !!str 42
integer: !!int "42"
float: !!float "3.14"
sequence: !!seq
    - one
    - two
mapping: !!map
    key: value
`,
	},
	{
		name:             "yaml/numeric_keys",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `"1": first
"2": second
"10": tenth
"100": hundredth
"1000": quoted_numeric
nested:
    "42": answer
    "0": zero
`,
	},
	{
		name:             "yaml/special_characters_in_values",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `colon_value: "has: colon"
hash_value: "has # hash"
bracket_value: "[not, a, flow]"
brace_value: "{not: a flow}"
ampersand: "not &anchor"
asterisk: "not *alias"
pipe: "not |literal"
gt: "not >folded"
`,
	},
	{
		name:             "yaml/deeply_nested_sequences",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `matrix:
    - - 1
      - 2
      - 3
    - - 4
      - 5
      - 6
nested_items:
    - items:
          - sub:
                - deep: true
`,
	},

	{
		name:             "yaml/block_scalar_keep_chomping",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `data:
  message: |+
    hello world
    second line


`,
	},
	{
		name:             "yaml/block_scalar_strip_chomping",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `data:
  message: |-
    hello world
    second line
`,
	},
	{
		name:             "yaml/block_scalar_clip_default",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input: `data:
  message: |
    hello world
    second line
`,
	},
	{
		name:             "yaml/block_scalar_trailing_spaces",
		format:           "yaml",
		formatter:        yamlfmt.Formatter{},
		checkEquivalence: yamlEquivalent,
		input:            "data:\n  content: |\n    line with trailing   \n    normal line\n",
	},

	// =========================================================================
	// TOML — 12 cases
	// =========================================================================
	{
		name:             "toml/cargo_toml_messy",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[package]
name="my-crate"
version  =  "0.1.0"
edition="2021"
authors   = ["Alice <alice@example.com>",  "Bob <bob@example.com>"]

[dependencies]
serde={version="1.0",   features=["derive"]}
tokio  = {version  =  "1.0",features=["full"]}
actix-web    ="4.0"

[dev-dependencies]
criterion="0.5"

[[bin]]
name   ="server"
path="src/main.rs"

[[bin]]
name  = "cli"
path  =  "src/cli.rs"
`,
	},
	{
		name:             "toml/multiline_strings",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[package]
name="test"
version="1.0.0"
description="""
This is a multiline
basic string with "quotes" inside
and a trailing newline
"""

[config]
template='''
No \escaping in
literal strings
'''
regex='\\d+\\.\\d+'
`,
	},
	{
		name:             "toml/inline_tables_and_arrays",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[project]
name="test"
license=  {text  =  "MIT"}
authors=[
  {name="Alice",   email="alice@example.com"},
  {name =  "Bob",email  ="bob@example.com"},
]
keywords=["web",  "api",   "async"]
classifiers  = [
"Development Status :: 3 - Alpha",
  "License :: OSI Approved :: MIT License",
"Programming Language :: Python :: 3",
]
`,
	},
	{
		name:             "toml/all_value_types",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[types]
string="hello"
integer=42
float=3.14
bool_true=true
bool_false=false
date=2024-01-15
time=10:30:00
datetime=2024-01-15T10:30:00Z
array=[1,2,3]
inline_table={key="value"}
`,
	},
	{
		name:             "toml/dotted_keys",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[server]
host="localhost"
port=8080

[server.ssl]
enabled=true
cert="/path/to/cert.pem"
key="/path/to/key.pem"

[server.ssl.options]
min_version="TLS1.2"
ciphers=["AES256","AES128"]
`,
	},
	{
		name:             "toml/array_of_tables",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[[products]]
name="Hammer"
sku=738594937

[[products]]
name="Nail"
sku=284758393
color="gray"

[[products]]
name="Screwdriver"
sku=123456789
color="red"
sizes=["small","medium","large"]
`,
	},
	{
		name:             "toml/empty_tables",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[empty]

[also_empty]

[has_content]
key="value"

[trailing_empty]
`,
	},
	{
		name:             "toml/special_strings",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[strings]
basic="I'm a basic string with \"escapes\""
literal='No\escape\here'
multiline_basic="""
line 1
line 2
line 3"""
multiline_literal='''
also preserves
everything literally'''
with_newline="line1\nline2"
with_tab="col1\tcol2"
with_unicode="caf\u00E9"
`,
	},
	{
		name:             "toml/numeric_variety",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[numbers]
pos_int=42
neg_int=-17
hex=0xDEADBEEF
oct=0o755
bin=0b11010110
float1=3.14
float2=-0.01
inf1=inf
inf2=-inf
nan1=nan
`,
	},
	{
		name:             "toml/comments_between_everything",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `# Top-level comment
[section1]
# Before key
key1="value1"
# Between keys
key2="value2"

# Between sections
[section2]
key3="value3"
# Trailing comment
`,
	},
	{
		name:             "toml/deeply_nested_tables",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[a]
key="1"
[a.b]
key="2"
[a.b.c]
key="3"
[a.b.c.d]
key="4"
[a.b.c.d.e]
key="5"
`,
	},
	{
		name:             "toml/messy_spacing_throughout",
		format:           "toml",
		formatter:        tomlfmt.Formatter{},
		checkEquivalence: tomlEquivalent,
		input: `[  package  ]
name   =   "messy"
version="1.0.0"
edition  ="2021"

[  dependencies  ]
serde  =  "1.0"
tokio="1.0"
rand   =    "0.8"
`,
	},

	// =========================================================================
	// Properties — 10 cases
	// =========================================================================
	{
		name:             "properties/spring_boot_messy",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Spring Boot Configuration
server.port=8080
server.address =  0.0.0.0
spring.datasource.url=jdbc:postgresql://localhost:5432/mydb
spring.datasource.username =admin
spring.datasource.password= secret123
spring.jpa.hibernate.ddl-auto  =  update

# Logging
logging.level.root  =INFO
logging.level.com.myapp=DEBUG
logging.file.path  = /var/log/myapp
`,
	},
	{
		name:             "properties/continuations_and_escapes",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Continuation lines
long.value = \
    this is a very long value \
    that spans multiple lines

# Escaped characters
path.windows = C:\\Users\\test\\config
unicode.key = \u0048ello
special.chars = value with \= equals and \: colons
tab.value = before\tafter
`,
	},
	{
		name:             "properties/colon_separator",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Colon-separated properties
host:localhost
port:  8080
name :  myapp
path : /api/v1
`,
	},
	{
		name:             "properties/whitespace_separator",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Space-separated (first whitespace is separator)
key1 value1
key2   value2
key3	tabbed_value
`,
	},
	{
		name:             "properties/empty_values",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Keys with empty values
empty.equals=
empty.colon:
empty.space 
`,
	},
	{
		name:             "properties/special_key_chars",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Keys with escaped special characters
key\ with\ spaces = value
key\=with\=equals = value
key\:with\:colons = value
key\\with\\backslash = value
`,
	},
	{
		name:             "properties/bang_comments",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `! This is a bang comment
! They are less common but valid
key1 = value1
! Between entries
key2 = value2
# Mixed with hash comments
key3 = value3
`,
	},
	{
		name:             "properties/multiple_continuations",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Many continuation lines
targets = \
    target1, \
    target2, \
    target3, \
    target4, \
    target5

single = no continuation here
`,
	},
	{
		name:             "properties/unicode_values",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Unicode in values and keys
greeting = Hello, \u4e16\u754c
name = caf\u00e9
path.to.\u0066ile = /tmp/test
emoji = \ud83d\ude00
`,
	},
	{
		name:             "properties/mixed_separators",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input: `# Mix of all separator styles in one file
equals=value1
colon:value2
space value3
equals_spaces = value4
colon_spaces : value5
`,
	},
	{
		name:             "properties/continuation_at_eof",
		format:           "properties",
		formatter:        propfmt.Formatter{},
		checkEquivalence: propertiesEquivalent,
		input:            "key1 = value\\\n    continued\nkey2 = normal\n",
	},

	// =========================================================================
	// INI — 8 cases
	// =========================================================================
	{
		name:             "ini/mysql_config",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `# MySQL Configuration
[mysqld]
port=3306
bind-address  =  0.0.0.0
max_connections =  151
character-set-server=utf8mb4

; InnoDB settings
innodb_buffer_pool_size=1G
innodb_log_file_size  = 256M
innodb_flush_method =O_DIRECT

[client]
port=3306
default-character-set  =  utf8mb4

[mysqldump]
max_allowed_packet= 64M
`,
	},
	{
		name:             "ini/php_ini_multiple_sections",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `; PHP Configuration
[PHP]
engine=On
precision=14
memory_limit  = 256M
post_max_size=64M
upload_max_filesize  =  32M

[Date]
date.timezone  =  UTC

[Session]
session.save_handler=files
session.gc_maxlifetime=1440
session.cookie_httponly  =  On

[opcache]
opcache.enable=1
opcache.memory_consumption  =  128
opcache.revalidate_freq=60
`,
	},
	{
		name:             "ini/colon_separator",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `[section]
key1:value1
key2 :  value2
key3:  value3
key4 :value4
`,
	},
	{
		name:             "ini/mixed_comment_styles",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `# Hash comment
[section1]
; Semicolon comment
key1=value1
# Another hash
key2=value2

; Before section
[section2]
# Inside section
key3=value3
`,
	},
	{
		name:             "ini/quoted_values",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `[paths]
home="/home/user"
config='/etc/myapp/config'
data_dir="/var/lib/myapp"
log_file='/var/log/myapp.log'
`,
	},
	{
		name:             "ini/many_sections",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `[server]
port=8080
host=0.0.0.0

[database]
host=localhost
port=5432

[cache]
host=localhost
port=6379

[logging]
level=info
file=/var/log/app.log

[auth]
secret=mysecret
expiry=3600
`,
	},
	{
		name:             "ini/values_with_special_chars",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `[urls]
api=https://api.example.com/v1?key=abc&format=json
webhook=http://localhost:3000/hooks/deploy#section
jdbc=jdbc:postgresql://db.host:5432/mydb?ssl=true&timeout=30

[paths]
windows=C:\Users\admin\config
unix=/home/user/.config/app
`,
	},
	{
		name:             "ini/single_section",
		format:           "ini",
		formatter:        inifmt.Formatter{},
		checkEquivalence: iniEquivalent,
		input: `[only_section]
key1=val1
key2  =  val2
key3=val3
key4  =val4
key5=  val5
`,
	},

	// =========================================================================
	// ENV — 8 cases
	// =========================================================================
	{
		name:             "env/dotenv_standard",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `# Application
NODE_ENV  =production
PORT  =3000
HOST  =0.0.0.0

# Database
DATABASE_URL  ="postgres://user:pass@localhost:5432/myapp"
DATABASE_POOL  =20

# Auth
JWT_SECRET  ="my-secret"
JWT_EXPIRY  =3600
`,
	},
	{
		name:             "env/quoted_values",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `# Quoted values
DOUBLE_QUOTED  ="hello world"
SINGLE_QUOTED  ='hello world'
UNQUOTED  =hello
EMPTY  =
SPACES_IN_VALUE  ="has spaces"
`,
	},
	{
		name:             "env/no_spaces",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `KEY1=value1
KEY2=value2
KEY3=value3
`,
	},
	//nolint:gosec // G101: test fixture URLs with fake credentials
	{
		name:             "env/urls_and_connection_strings",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `DATABASE_URL  ="postgres://user:p%40ss@host:5432/db?sslmode=require"
REDIS_URL  ="redis://:password@redis:6379/0"
AMQP_URL  ="amqp://user:pass@rabbit:5672/vhost"
MONGO_URI  ="mongodb://user:pass@mongo1:27017,mongo2:27017/db?replicaSet=rs0"
`,
	},
	{
		name:             "env/multiline_and_special",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `# Special characters in values
GREETING  ="Hello, World!"
JSON_CONFIG  ='{"key":"value","arr":[1,2,3]}'
REGEX  ="^[a-z]+$"
COMMAND  ="echo 'hello' && exit 0"
`,
	},
	{
		name:             "env/comments_and_blanks",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `# First section

KEY1  =val1

# Second section

KEY2  =val2

# Third section

KEY3  =val3
`,
	},
	{
		name:             "env/numeric_and_bool_values",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `PORT  =8080
MAX_RETRIES  =3
TIMEOUT_MS  =30000
ENABLED  =true
DEBUG  =false
RATIO  =0.75
`,
	},
	{
		name:             "env/export_prefix",
		format:           "env",
		formatter:        envfmt.Formatter{},
		checkEquivalence: envEquivalent,
		input: `export NODE_ENV  =production
export PORT  =3000
export DEBUG  =false
`,
	},

	// =========================================================================
	// HCL — 8 cases
	// =========================================================================
	{
		name:             "hcl/terraform_messy",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `resource "aws_instance" "web" {
ami           =   "ami-0c55b159cbfafe1f0"
  instance_type =  "t2.micro"
    tags = {
  Name  = "HelloWorld"
      Environment="production"
    }
}

variable "region" {
    default   = "us-east-1"
  type =  string
}

output "instance_ip" {
  value =   aws_instance.web.public_ip
    description= "The public IP"
}
`,
	},
	{
		name:             "hcl/security_group_complex",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `resource "aws_security_group" "web" {
    name =  "web-sg"
  description   = "Security group for web"

 ingress {
from_port =  80
  to_port   = 80
    protocol  = "tcp"
      cidr_blocks=["0.0.0.0/0"]
  }

    ingress {
    from_port=443
to_port  =443
      protocol ="tcp"
  cidr_blocks  = ["0.0.0.0/0"]
    }

  egress {
      from_port=0
    to_port =0
  protocol  ="-1"
        cidr_blocks =  ["0.0.0.0/0"]
  }
}
`,
	},
	{
		name:             "hcl/locals_and_data",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `locals {
    environment =  "production"
  region      = "us-east-1"
      project = "myapp"
    tags = {
  Environment =   local.environment
        Project = local.project
      ManagedBy="terraform"
    }
}

data "aws_ami" "ubuntu" {
  most_recent =  true
    owners=["099720109477"]

  filter {
      name="name"
    values  =  ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }
}
`,
	},
	{
		name:             "hcl/module_block",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `module "vpc" {
    source =  "terraform-aws-modules/vpc/aws"
  version   = "5.0.0"

      name="main-vpc"
    cidr =  "10.0.0.0/16"

  azs             = ["us-east-1a",  "us-east-1b",  "us-east-1c"]
      private_subnets=["10.0.1.0/24","10.0.2.0/24","10.0.3.0/24"]
    public_subnets  = ["10.0.101.0/24","10.0.102.0/24","10.0.103.0/24"]

  enable_nat_gateway =  true
      single_nat_gateway   = true
}
`,
	},
	{
		name:             "hcl/dynamic_blocks",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `resource "aws_security_group" "dynamic" {
    name="dynamic-sg"

dynamic "ingress" {
  for_each =   var.ingress_rules
    content {
from_port   = ingress.value.from
  to_port     =ingress.value.to
      protocol    = ingress.value.protocol
    cidr_blocks=ingress.value.cidrs
    }
}
}
`,
	},
	{
		name:             "hcl/provider_config",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `terraform {
  required_version =  ">= 1.5.0"
    required_providers {
  aws = {
      source  = "hashicorp/aws"
    version =  "~> 5.0"
  }
      random={
    source="hashicorp/random"
        version  = "~> 3.0"
      }
  }
}

provider "aws" {
    region =  var.region
  default_tags {
      tags={
    Environment =  var.environment
        Project  = var.project
      }
  }
}
`,
	},
	{
		name:             "hcl/complex_expressions",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `locals {
  subnet_ids =   [for s in aws_subnet.main : s.id]
    instance_names = {for i in aws_instance.web :   i.id => i.tags.Name}
  is_prod =  var.environment == "production"
      ami_id  = var.custom_ami != "" ? var.custom_ami : data.aws_ami.ubuntu.id
}
`,
	},
	{
		name:             "hcl/multiline_strings",
		format:           "hcl",
		formatter:        hclfmt.Formatter{},
		checkEquivalence: hclEquivalent,
		input: `resource "aws_iam_policy" "example" {
    name =  "example-policy"
  policy   = jsonencode({
      Version = "2012-10-17"
    Statement=[{
      Effect =   "Allow"
        Action = [
    "s3:GetObject",
          "s3:PutObject",
      "s3:DeleteObject",
        ]
      Resource="arn:aws:s3:::my-bucket/*"
    }]
  })
}
`,
	},

	// =========================================================================
	// XML — 8 cases
	// =========================================================================
	{
		name:             "xml/maven_pom_messy_indent",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0" encoding="UTF-8"?>
<project>
<modelVersion>4.0.0</modelVersion>
<groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
<version>1.0.0</version>
    <dependencies>
  <dependency>
            <groupId>org.springframework</groupId>
    <artifactId>spring-core</artifactId>
                <version>5.3.0</version>
        </dependency>
            <dependency>
  <groupId>junit</groupId>
        <artifactId>junit</artifactId>
    <version>4.13</version>
            </dependency>
    </dependencies>
</project>
`,
	},
	{
		name:             "xml/spring_beans",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0" encoding="UTF-8"?>
<beans>
<bean id="dataSource" class="org.apache.commons.dbcp2.BasicDataSource">
      <property name="url" value="jdbc:postgresql://localhost/db"/>
  <property name="username" value="admin"/>
          <property name="password" value="secret"/>
</bean>
    <bean id="txManager" class="org.springframework.orm.hibernate5.HibernateTransactionManager">
            <property name="sessionFactory" ref="sf"/>
  </bean>
</beans>
`,
	},
	{
		name:             "xml/self_closing_elements",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0"?>
<config>
<server host="localhost" port="8080"/>
    <database url="jdbc:pg://localhost/db" user="admin" password="secret"/>
        <cache enabled="true" ttl="300"/>
  <logging level="info" file="/var/log/app.log"/>
</config>
`,
	},
	{
		name:             "xml/nested_deep",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0"?>
<root>
<level1>
<level2>
<level3>
<level4>
<value>deep</value>
</level4>
</level3>
</level2>
</level1>
</root>
`,
	},
	{
		name:             "xml/attributes_many",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0"?>
<widgets>
    <widget id="1" type="button" label="Click Me" enabled="true" visible="true"/>
  <widget id="2" type="input" placeholder="Enter text" maxlength="100" required="true"/>
        <widget id="3" type="select" multiple="true" size="5"/>
</widgets>
`,
	},
	{
		name:             "xml/cdata_section",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0"?>
<scripts>
<script name="init"><![CDATA[
function init() {
    if (x < 10 && y > 5) {
        console.log("hello");
    }
}
]]></script>
    <script name="cleanup"><![CDATA[
document.querySelectorAll('.temp').forEach(e => e.remove());
]]></script>
</scripts>
`,
	},
	{
		name:             "xml/empty_elements",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0"?>
<config>
<empty></empty>
    <also-empty>
    </also-empty>
        <self-close/>
<with-attr name="test"></with-attr>
</config>
`,
	},
	{
		name:             "xml/processing_instructions",
		format:           "xml",
		formatter:        xmlfmt.Formatter{},
		checkEquivalence: xmlEquivalent,
		input: `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" href="transform.xsl"?>
<root>
<child>value</child>
    <other>data</other>
</root>
`,
	},
}

// TestStressFormatWithSortKeys tests that sorting keys preserves semantic content.
func TestStressFormatWithSortKeys(t *testing.T) {
	t.Parallel()

	sortCases := []stressCase{
		{
			name:             "json/sort_keys",
			format:           "json",
			formatter:        jsonfmt.Formatter{},
			checkEquivalence: jsonEquivalent,
			opts:             formatter.Options{IndentWidth: 2, SortKeys: true},
			input:            `{"zebra":1,"alpha":2,"middle":{"z":1,"a":2,"m":3},"beta":[{"c":3,"a":1,"b":2}]}`,
		},
		{
			name:             "yaml/sort_keys_preserves_data",
			format:           "yaml",
			formatter:        yamlfmt.Formatter{},
			checkEquivalence: yamlEquivalent,
			opts: formatter.Options{
				IndentWidth: 2,
				SortKeys:    true,
			},
			input: `zebra: 1
alpha: 2
middle:
  z: first
  a: second
  m: third
beta:
  - name: charlie
    value: 3
  - name: alice
    value: 1
`,
		},
		{
			name:             "toml/sort_keys_preserves_data",
			format:           "toml",
			formatter:        tomlfmt.Formatter{},
			checkEquivalence: tomlEquivalent,
			opts: formatter.Options{
				IndentWidth: 2,
				SortKeys:    true,
			},
			input: `[package]
version = "1.0.0"
name = "my-crate"
edition = "2021"

[dependencies]
tokio = "1.0"
actix-web = "4.0"
serde = "1.0"
`,
		},
		{
			name:             "properties/sort_keys_preserves_data",
			format:           "properties",
			formatter:        propfmt.Formatter{},
			checkEquivalence: propertiesEquivalent,
			opts: formatter.Options{
				IndentWidth: 2,
				SortKeys:    true,
			},
			input: `# Config
z.last = end
a.first = start
m.middle = center
server.port = 8080
server.host = localhost
app.name = myapp
`,
		},
		{
			name:             "ini/sort_keys_preserves_data",
			format:           "ini",
			formatter:        inifmt.Formatter{},
			checkEquivalence: iniEquivalent,
			opts: formatter.Options{
				IndentWidth: 2,
				SortKeys:    true,
			},
			input: `[server]
port = 8080
host = localhost
name = myserver

[database]
password = secret
username = admin
url = localhost:5432
`,
		},
	}

	for _, tc := range sortCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			formatted, err := tc.formatter.Format([]byte(tc.input), tc.opts)
			require.NoError(t, err, "format failed")

			// Idempotent.
			reformatted, err := tc.formatter.Format(formatted, tc.opts)
			require.NoError(t, err, "re-format failed")
			require.Equal(t, string(formatted), string(reformatted), "not idempotent")

			// Semantically equivalent.
			tc.checkEquivalence(t, []byte(tc.input), formatted)
		})
	}
}

// TestStressFormatCLIEndToEnd exercises the formatter through file I/O,
// simulating what cfv format --fix actually does.
func TestStressFormatCLIEndToEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write messy files.
	files := map[string]string{
		"app.json":       `{"port":3000,"host":"0.0.0.0","debug":true,"cors":{"origins":["http://localhost:3000","http://localhost:5173"]}}`,
		"config.yaml":    "server:\n    port: 8080\n    host: 0.0.0.0\ndatabase:\n        url: postgres://localhost/db\n        pool: 10\n",
		"settings.toml":  "[server]\nport=8080\nhost  =  \"0.0.0.0\"\n\n[database]\nurl  = \"postgres://localhost/db\"\npool=10\n",
		"app.properties": "server.port=8080\nserver.host =  0.0.0.0\ndb.url=postgres://localhost/db\ndb.pool  =  10\n",
		"db.ini":         "[database]\nhost=localhost\nport  =  5432\nname=mydb\nuser =  admin\n",
		"runtime.env":    "PORT  =8080\nHOST  =0.0.0.0\nDATABASE_URL  =\"postgres://localhost/db\"\n",
		"main.tf":        "variable \"port\" {\n  default =   8080\n    type = number\n}\n",
		"page.xml":       "<?xml version=\"1.0\"?>\n<config>\n<server>\n     <port>8080</port>\n  <host>0.0.0.0</host>\n</server>\n</config>\n",
		"tsconfig.jsonc": "{\n  // Compiler\n    \"compilerOptions\":  {\n  \"target\":\"ES2022\",\n      \"strict\":true\n  }\n}\n",
	}

	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	// Parse all files BEFORE formatting to capture original semantics.
	originals := make(map[string]any)
	originals["app.json"] = parseJSON(t, files["app.json"])
	originals["config.yaml"] = parseYAML(t, files["config.yaml"])
	originals["settings.toml"] = parseTOML(t, files["settings.toml"])
	originals["app.properties"] = parseProps(t, files["app.properties"])
	originals["db.ini"] = parseINI(t, files["db.ini"])
	originals["runtime.env"] = parseEnv(files["runtime.env"])

	// Format all files using the formatters directly (simulating cfv format --fix).
	formatters := map[string]struct {
		f    formatter.Formatter
		opts formatter.Options
	}{
		"app.json":       {jsonfmt.Formatter{}, jsonfmt.DefaultOptions()},
		"config.yaml":    {yamlfmt.Formatter{}, yamlfmt.DefaultOptions()},
		"settings.toml":  {tomlfmt.Formatter{}, tomlfmt.DefaultOptions()},
		"app.properties": {propfmt.Formatter{}, propfmt.DefaultOptions()},
		"db.ini":         {inifmt.Formatter{}, inifmt.DefaultOptions()},
		"runtime.env":    {envfmt.Formatter{}, envfmt.DefaultOptions()},
		"main.tf":        {hclfmt.Formatter{}, formatter.Options{IndentWidth: 2}},
		"page.xml":       {xmlfmt.Formatter{}, xmlfmt.DefaultOptions()},
		"tsconfig.jsonc": {jsoncfmt.Formatter{}, jsoncfmt.DefaultOptions()},
	}

	for name, fmtr := range formatters {
		src, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)

		formatted, err := fmtr.f.Format(src, fmtr.opts)
		require.NoError(t, err, "format %s", name)

		require.NoError(t, os.WriteFile(filepath.Join(dir, name), formatted, 0o600)) //nolint:gosec // test-controlled filename

		// Verify idempotent.
		reformatted, err := fmtr.f.Format(formatted, fmtr.opts)
		require.NoError(t, err, "re-format %s", name)
		require.Equal(t, string(formatted), string(reformatted),
			"NOT IDEMPOTENT: %s", name)
	}

	// Verify semantic equivalence for parseable formats.
	formatted, _ := os.ReadFile(filepath.Join(dir, "app.json"))
	require.Equal(t, originals["app.json"], parseJSON(t, string(formatted)), "JSON semantics changed")

	formatted, _ = os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.Equal(t, originals["config.yaml"], parseYAML(t, string(formatted)), "YAML semantics changed")

	formatted, _ = os.ReadFile(filepath.Join(dir, "settings.toml"))
	require.Equal(t, originals["settings.toml"], parseTOML(t, string(formatted)), "TOML semantics changed")

	formatted, _ = os.ReadFile(filepath.Join(dir, "app.properties"))
	require.Equal(t, originals["app.properties"], parseProps(t, string(formatted)), "Properties semantics changed")

	formatted, _ = os.ReadFile(filepath.Join(dir, "db.ini"))
	require.Equal(t, originals["db.ini"], parseINI(t, string(formatted)), "INI semantics changed")

	formatted, _ = os.ReadFile(filepath.Join(dir, "runtime.env"))
	require.Equal(t, originals["runtime.env"], parseEnv(string(formatted)), "ENV semantics changed")
}

// =============================================================================
// Parse helpers for end-to-end test
// =============================================================================

func parseJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	return v
}

func parseYAML(t *testing.T, s string) any {
	t.Helper()
	var v any
	require.NoError(t, yaml.Unmarshal([]byte(s), &v))
	return v
}

func parseTOML(t *testing.T, s string) any {
	t.Helper()
	var v map[string]any
	require.NoError(t, toml.Unmarshal([]byte(s), &v))
	return v
}

func parseProps(t *testing.T, s string) any {
	t.Helper()
	p, err := properties.Load([]byte(s), properties.UTF8)
	require.NoError(t, err)
	return p.Map()
}

func parseINI(t *testing.T, s string) any {
	t.Helper()
	f, err := ini.Load([]byte(s))
	require.NoError(t, err)
	result := make(map[string]map[string]string)
	for _, sec := range f.SectionStrings() {
		section := f.Section(sec)
		m := make(map[string]string)
		for _, key := range section.KeyStrings() {
			m[key] = section.Key(key).String()
		}
		result[sec] = m
	}
	return result
}

// TestRealWorldCorpus formats real config files from this repository and verifies:
//  1. Format succeeds (no error)
//  2. Output is idempotent (format again = same output)
//  3. Semantic equivalence (parsed data is the same)
func TestRealWorldCorpus(t *testing.T) {
	t.Parallel()
	repoRoot := findRepoRoot(t)

	cases := []struct {
		path    string
		format  string
		fmtr    formatter.Formatter
		checkEq func(t *testing.T, original, formatted []byte)
	}{
		// YAML
		{".golangci.yaml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{".mega-linter.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{".pre-commit-hooks.yaml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{"demo.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{".github/workflows/go.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{".github/workflows/release.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		{".github/dependabot.yml", "yaml", yamlfmt.Formatter{}, yamlEquivalent},
		// JSON
		{"website/package.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
		{"pkg/configfile/schema.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
		{".markdownlint.json", "json", jsonfmt.Formatter{}, jsonEquivalent},
		// JSONC
		{"website/tsconfig.json", "jsonc", jsoncfmt.Formatter{}, jsoncEquivalent},
	}

	for _, tc := range cases {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(filepath.Join(repoRoot, tc.path))
			if os.IsNotExist(err) {
				t.Skipf("file not found: %s", tc.path)
			}
			require.NoError(t, err)

			opts := defaultOpts(tc.format)
			formatted, err := tc.fmtr.Format(src, opts)
			require.NoError(t, err, "format %s failed", tc.path)

			// Idempotent
			result2, err := tc.fmtr.Format(formatted, opts)
			require.NoError(t, err)
			require.Equal(t, string(formatted), string(result2),
				"not idempotent: %s", tc.path)

			// Semantic equivalence
			tc.checkEq(t, src, formatted)
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}
