package gojust

import (
	"strings"
	"testing"
)

// TestAdversarial tries to break the parser with pathological inputs.
// Each test targets a specific parser boundary or ambiguity.
func TestAdversarial(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// === Minimal and empty inputs ===
		{"empty", "", false},
		{"just_newline", "\n", false},
		{"just_spaces", "   \n", false},
		{"just_tabs", "\t\t\t\n", false},
		{"just_comment", "# nothing\n", false},
		{"only_shebang", "#!/usr/bin/env just\n", false},
		{"shebang_no_newline", "#!/usr/bin/env just", false},

		// === Single character edge cases ===
		{"lone_colon", ":", true},
		{"lone_at", "@", true},
		{"lone_plus", "+", true},
		{"lone_star", "*", true},
		{"lone_dollar", "$", true},
		{"lone_equals", "=", true},
		{"lone_bang", "!", true},
		{"lone_lbrace", "{", true},
		{"lone_rbrace", "}", true},
		{"lone_lparen", "(", true},
		{"lone_rparen", ")", true},
		{"lone_lbracket", "[", true},
		{"lone_rbracket", "]", true},

		// === Recipe name edge cases ===
		{"recipe_underscore", "_:\n    echo done\n", false},
		{"recipe_single_char", "a:\n    echo done\n", false},
		{"recipe_with_dashes", "my-long-recipe-name:\n    echo done\n", false},
		{"recipe_with_numbers", "build123:\n    echo done\n", false},
		{"recipe_keyword_import", "import:\n    echo done\n", false},
		{"recipe_starts_with_underscore", "_private_recipe:\n    echo done\n", false},

		// === Assignment edge cases ===
		{"assign_empty_string", `v := ""` + "\n", false},
		{"assign_empty_raw", "v := ''\n", false},
		{"assign_single_char_string", `v := "x"` + "\n", false},
		{"assign_just_backtick", "v := `true`\n", false},
		{"assign_nested_parens", `v := ((("deep")))` + "\n", false},
		{"assign_many_concats", `v := "a" + "b" + "c" + "d" + "e" + "f" + "g" + "h" + "i" + "j" + "k" + "l" + "m" + "n" + "o" + "p"` + "\n", false},
		{"assign_many_paths", `v := "a" / "b" / "c" / "d" / "e" / "f" / "g" / "h"` + "\n", false},
		{"assign_mixed_ops", `v := "a" + "b" / "c" + "d" / "e"` + "\n", false},
		{"assign_leading_slash", `v := / "absolute"` + "\n", false},
		{"assign_leading_slash_chain", `v := / "a" / "b" / "c"` + "\n", false},

		// === String edge cases ===
		{"string_with_single_quote_inside", `v := "it's"` + "\n", false},
		{"raw_string_with_double_quote", "v := 'say \"hello\"'\n", false},
		{"string_with_backslash_at_end", `v := "path\\"` + "\n", false},
		{"string_unicode_emoji", `v := "\u{1F600}"` + "\n", false},
		{"string_unicode_null", `v := "\u{0000}"` + "\n", false},
		{"string_unicode_max_bmp", `v := "\u{FFFF}"` + "\n", false},
		{"empty_indented_string", "v := \"\"\"\n\"\"\"\n", false},
		{"empty_indented_raw", "v := '''\n'''\n", false},
		{"empty_backtick", "v := ``\n", false},
		{"multiline_raw_many_lines", "v := '\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n'\n", false},
		{"string_with_braces", `v := "hello { world }"` + "\n", false},
		{"string_with_double_braces", `v := "hello {{ world }}"` + "\n", false},

		// === Conditional edge cases ===
		{"cond_same_values", `v := if "a" == "a" { "same" } else { "diff" }` + "\n", false},
		{"cond_empty_strings", `v := if "" == "" { "empty" } else { "not" }` + "\n", false},
		{"cond_nested_3_deep", `v := if "a" == "b" { "1" } else if "c" == "d" { "2" } else if "e" == "f" { "3" } else { "4" }` + "\n", false},
		{"cond_with_function", `v := if arch() == "x86_64" { "64" } else { "other" }` + "\n", false},
		{"cond_with_concat", `v := if "a" + "b" == "ab" { "yes" } else { "no" }` + "\n", false},
		{"cond_regex", `v := if "hello" =~ "^h" { "match" } else { "no" }` + "\n", false},
		{"cond_regex_mismatch", `v := if "hello" !~ "^z" { "good" } else { "bad" }` + "\n", false},
		{"cond_multiline", "v := if \"a\" == \"b\" {\n  \"yes\"\n} else {\n  \"no\"\n}\n", false},

		// === Recipe body edge cases ===
		{"body_empty_interpolation_var", "v := \"x\"\nbuild:\n    echo {{v}}\n", false},
		{"body_interpolation_function", "build:\n    echo {{arch()}}\n", false},
		{"body_interpolation_conditional", "build:\n    echo {{ if \"a\" == \"b\" { \"y\" } else { \"n\" } }}\n", false},
		{"body_interpolation_concat", "build:\n    echo {{\"a\" + \"b\" + \"c\"}}\n", false},
		{"body_interpolation_path", "build:\n    echo {{\"a\" / \"b\"}}\n", false},
		{"body_brace_escape", "build:\n    echo {{{{literal}}\n", false},
		{"body_only_interpolation", "build:\n    {{\"hello\"}}\n", false},
		{"body_text_then_interp", "build:\n    prefix {{\"middle\"}} suffix\n", false},
		{"body_multiple_interps", "build:\n    {{\"a\"}} {{\"b\"}} {{\"c\"}}\n", false},
		{"body_line_prefix_at", "build:\n    @echo quiet\n", false},
		{"body_line_prefix_dash", "build:\n    -echo may_fail\n", false},
		{"body_line_prefix_at_dash", "build:\n    @-echo both\n", false},
		{"body_line_prefix_dash_at", "build:\n    -@echo both\n", false},
		{"body_shebang_bash", "build:\n    #!/usr/bin/env bash\n    set -e\n    echo done\n", false},
		{"body_shebang_python", "build:\n    #!/usr/bin/env python3\n    print('hello')\n", false},
		{"body_shebang_node", "build:\n    #!/usr/bin/env node\n    console.log('hello')\n", false},
		{"body_many_lines", "build:\n" + strings.Repeat("    echo line\n", 50), false},
		{"body_empty_lines_between", "build:\n    echo start\n\n\n\n    echo end\n", false},
		{"body_tab_indent", "build:\n\techo done\n", false},

		// === Parameter edge cases ===
		{"param_all_types", "build a b='default' $c +d:\n    echo done\n", false},
		{"param_star_variadic", "build *args:\n    echo done\n", false},
		{"param_plus_variadic", "build +args:\n    echo done\n", false},
		{"param_dollar_export", "build $VAR:\n    echo done\n", false},
		{"param_dollar_variadic", "build +$VAR:\n    echo done\n", false},
		{"param_default_string", "build mode=\"release\":\n    echo done\n", false},
		{"param_default_raw", "build mode='release':\n    echo done\n", false},
		{"param_default_backtick", "build mode=`echo release`:\n    echo done\n", false},
		{"param_default_function", "build mode=arch():\n    echo done\n", false},
		{"param_many", "build a b c d e f g h:\n    echo done\n", false},

		// === Dependency edge cases ===
		{"dep_simple", "a:\n    echo a\nb: a\n    echo b\n", false},
		{"dep_multiple", "a:\n    echo a\nb:\n    echo b\nc: a b\n    echo c\n", false},
		{"dep_with_arg", "a x:\n    echo done\nb: (a \"hello\")\n    echo done\n", false},
		{"dep_with_multiple_args", "a x y:\n    echo done\nb: (a \"hello\" \"world\")\n    echo done\n", false},
		{"dep_subsequent", "a:\n    echo a\nb:\n    echo b\nc: a && b\n    echo c\n", false},
		{"dep_subsequent_with_args", "a x:\n    echo a\nb: && (a \"hello\")\n    echo b\n", false},
		{"dep_many", "a:\n    echo a\nb:\n    echo b\nc:\n    echo c\nd:\n    echo d\ne: a b c d\n    echo e\n", false},

		// === Attribute edge cases ===
		{"attr_private", "[private]\nbuild:\n    echo done\n", false},
		{"attr_no_cd", "[no-cd]\nbuild:\n    echo done\n", false},
		{"attr_confirm_no_arg", "[confirm]\nbuild:\n    echo done\n", false},
		{"attr_confirm_with_arg", "[confirm('sure?')]\nbuild:\n    echo done\n", false},
		{"attr_group_shorthand", "[group: 'ci']\nbuild:\n    echo done\n", false},
		{"attr_doc", "[doc('Build the project')]\nbuild:\n    echo done\n", false},
		{"attr_multiple_on_lines", "[private]\n[no-cd]\n[linux]\nbuild:\n    echo done\n", false},
		{"attr_multiple_inline", "[private, no-cd, linux]\nbuild:\n    echo done\n", false},
		{"attr_with_kv", "[env(name=\"FOO\", value=\"bar\")]\nbuild:\n    echo done\n", false},
		{"attr_os_linux", "[linux]\nbuild:\n    echo done\n", false},
		{"attr_os_macos", "[macos]\nbuild:\n    echo done\n", false},
		{"attr_os_windows", "[windows]\nbuild:\n    echo done\n", false},
		{"attr_script", "[script('bash')]\nbuild:\n    echo done\n", false},

		// === Setting edge cases ===
		{"setting_bare_bool", "set dotenv-load\n", false},
		{"setting_true", "set dotenv-load := true\n", false},
		{"setting_false", "set dotenv-load := false\n", false},
		{"setting_string", "set tempdir := '/tmp'\n", false},
		{"setting_list", "set shell := ['bash', '-cu']\n", false},
		{"setting_list_trailing_comma", "set shell := ['bash', '-cu',]\n", false},
		{"setting_keyword_export", "set export\n", false},
		{"setting_keyword_quiet", "set quiet\n", false},
		{"setting_all_bools", "set dotenv-load\nset export\nset positional-arguments\nset quiet\nset fallback\nset unstable\n", false},

		// === Import/module edge cases ===
		{"import_simple", "import 'foo.just'\nbuild:\n    echo done\n", false},
		{"import_optional", "import? 'maybe.just'\nbuild:\n    echo done\n", false},
		{"mod_simple", "mod foo\nbuild:\n    echo done\n", false},
		{"mod_optional", "mod? foo\nbuild:\n    echo done\n", false},
		{"mod_with_path", "mod foo 'path/to/foo.just'\nbuild:\n    echo done\n", false},
		{"multiple_imports", "import 'a.just'\nimport 'b.just'\nimport? 'c.just'\nbuild:\n    echo done\n", false},

		// === Function call edge cases ===
		{"func_no_args", "v := arch()\n", false},
		{"func_one_arg", "v := env_var(\"HOME\")\n", false},
		{"func_two_args", "v := env_var_or_default(\"X\", \"y\")\n", false},
		{"func_three_args", "v := replace(\"a.b\", \".\", \"-\")\n", false},
		{"func_nested", "v := replace(replace(\"a.b.c\", \".\", \"-\"), \"-\", \"_\")\n", false},
		{"func_in_concat", "v := arch() + \"-\" + os()\n", false},
		{"func_in_path", "v := justfile_directory() / \"src\"\n", false},
		{"func_in_conditional", "v := if arch() == \"x86_64\" { \"64\" } else { \"32\" }\n", false},

		// === Eager/export edge cases ===
		{"export_simple", "export FOO := \"bar\"\n", false},
		{"eager_simple", "eager val := `cmd`\n", false},
		{"eager_export", "eager export TOKEN := `cmd`\n", false},
		{"export_eager", "export eager TOKEN := `cmd`\n", false},
		{"unexport", "export FOO := \"bar\"\nunexport FOO\n", false},

		// === Function definition edge cases ===
		{"funcdef_one_param", "f(x) := x\n", false},
		{"funcdef_two_params", "f(x, y) := x + y\n", false},
		{"funcdef_three_params", "f(a, b, c) := a + b + c\n", false},
		{"funcdef_with_conditional", "f(x) := if x == \"a\" { \"yes\" } else { \"no\" }\n", false},
		{"funcdef_multiline", "f(x) := if x == \"a\" {\n  \"yes\"\n} else {\n  \"no\"\n}\n", false},

		// === Alias edge cases ===
		{"alias_simple", "build:\n    echo done\nalias b := build\n", false},
		{"alias_with_attr", "[private]\nbuild:\n    echo done\nalias b := build\n", false},

		// === Comment edge cases ===
		{"comment_only", "# just a comment\n", false},
		{"comment_before_recipe", "# docs\nbuild:\n    echo done\n", false},
		{"comment_between_recipes", "a:\n    echo a\n# between\nb:\n    echo b\n", false},
		{"comment_after_setting", "set dotenv-load # not a comment in just but we handle it\n", false},
		{"comment_with_special_chars", "# !@#$%^&*(){}[]|\\:;\"'<>,.?/~`\n", false},

		// === Whitespace edge cases ===
		{"trailing_newlines", "build:\n    echo done\n\n\n\n", false},
		{"leading_newlines", "\n\n\nbuild:\n    echo done\n", false},
		{"blank_lines_everywhere", "\n\nv := \"x\"\n\n\nbuild:\n    echo done\n\n\n", false},
		{"spaces_in_expressions", "v  :=  \"a\"  +  \"b\"\n", false},

		// === Line continuation ===
		{"line_cont_in_assign", "v := \"a\" + \\\n  \"b\"\n", false},
		{"line_cont_crlf", "v := \"a\" + \\\r\n  \"b\"\n", false},

		// === Combination stress ===
		{"everything_together", `#!/usr/bin/env just
set shell := ['bash', '-cu']
set dotenv-load
set export

version := "1.0.0"
export DB := "postgres"
eager token := ` + "`echo secret`" + `

greeting(name) := "Hello, " + name + "!"

# Build the project
[group: 'build']
[doc('Build everything')]
build target='all' mode="debug":
    #!/usr/bin/env bash
    echo "Building {{target}} in {{mode}}"
    echo "Version: {{version}}"

[private]
test +args: build
    echo {{args}}

deploy env='staging': build (test "integration")
    echo "Deploying to {{env}}"

all: build && test deploy
    echo "All done"

@quiet-recipe:
    echo quiet

alias b := build
alias t := test

import? 'extra.just'
mod? tools
`,
			false,
		},

		// === Should-fail cases ===
		{"unterminated_string", `v := "unterminated`, true},
		{"unterminated_raw", "v := 'unterminated", true},
		{"unterminated_backtick", "v := `unterminated", true},
		{"unterminated_indented_string", "v := \"\"\"\nunterminated", true},
		{"unterminated_indented_raw", "v := '''\nunterminated", true},
		{"unterminated_indented_backtick", "v := ```\nunterminated", true},
		{"missing_colon", "build\n    echo done\n", true},
		{"bad_escape", `v := "\q"`, true},
		{"bad_unicode_no_brace", `v := "\u0041"`, true},
		{"bad_unicode_invalid", `v := "\u{ZZZZ}"`, true},
		{"bad_attribute", "[123]\nbuild:\n    echo done\n", true},
		{"unclosed_attribute", "[private\nbuild:\n    echo done\n", true},
		{"missing_assign_value", "v :=\n", true},
		{"export_no_assign", "export FOO\n", true},
		{"unexpected_token", "??? bad\n", true},
		{"unclosed_paren_dep", "build: (dep \"arg\"\n    echo done\n", true},
		{"unclosed_function", "v := env_var(\"HOME\"\n", true},
		{"cond_missing_else", `v := if "a" == "b" { "y" }`, true},
		{"cond_missing_operator", `v := if "a" { "y" } else { "n" }`, true},
		{"cond_missing_brace", `v := if "a" == "b" "y" } else { "n" }`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := Parse([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Validate shouldn't panic
			_ = jf.Validate()
		})
	}
}
