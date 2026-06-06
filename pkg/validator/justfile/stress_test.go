package gojust

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// stressGen is a second independent generator focused on adversarial
// combinations and boundary conditions rather than typical usage.
type stressGen struct {
	rng *rand.Rand
	ids int
}

func newStressGen(seed int64) *stressGen {
	return &stressGen{rng: rand.New(rand.NewPCG(uint64(seed), uint64(seed)))}
}

func (g *stressGen) id() string {
	g.ids++
	bases := []string{"a", "b", "c", "foo", "bar", "x", "y", "_z", "my-thing", "do_it"}
	return fmt.Sprintf("%s%d", bases[g.rng.IntN(len(bases))], g.ids)
}

func (g *stressGen) oneOf(choices ...string) string {
	return choices[g.rng.IntN(len(choices))]
}

func (g *stressGen) simpleStr() string {
	return g.oneOf(
		`"simple"`,
		`'raw'`,
		fmt.Sprintf(`"id_%d"`, g.rng.IntN(1000)),
		fmt.Sprintf(`'raw_%d'`, g.rng.IntN(1000)),
		`'has spaces'`,
		`"esc\nnewline"`,
		`"esc\ttab"`,
	)
}

func (g *stressGen) val() string {
	return g.oneOf(
		g.simpleStr(),
		g.id(),
		fmt.Sprintf("%s()", g.oneOf("arch", "os", "os_family", "num_cpus")),
		fmt.Sprintf(`env_var(%s)`, g.simpleStr()),
		fmt.Sprintf(`env_var_or_default(%s, %s)`, g.simpleStr(), g.simpleStr()),
		fmt.Sprintf("(%s)", g.simpleStr()),
		fmt.Sprintf("`echo %d`", g.rng.IntN(100)),
	)
}

func (g *stressGen) expr(depth int) string {
	if depth > 2 {
		return g.val()
	}
	return g.oneOf(
		g.val(),
		g.expr(depth+1)+" + "+g.expr(depth+1),
		g.expr(depth+1)+" / "+g.expr(depth+1),
		g.expr(depth+1)+" && "+g.expr(depth+1),
		g.expr(depth+1)+" || "+g.expr(depth+1),
		fmt.Sprintf(`if %s == %s { %s } else { %s }`, g.val(), g.val(), g.val(), g.val()),
		fmt.Sprintf(`if %s != %s { %s } else { %s }`, g.val(), g.val(), g.val(), g.val()),
		fmt.Sprintf(`if %s =~ %s { %s } else { %s }`, g.val(), g.simpleStr(), g.val(), g.val()),
		fmt.Sprintf(`replace(%s, %s, %s)`, g.simpleStr(), g.simpleStr(), g.simpleStr()),
		fmt.Sprintf(`trim(%s)`, g.simpleStr()),
		fmt.Sprintf(`uppercase(%s)`, g.simpleStr()),
	)
}

// Generators for each top-level item type

func (g *stressGen) genSettings(n int) string {
	var b strings.Builder
	settings := []string{
		"set dotenv-load\n",
		"set export\n",
		"set positional-arguments\n",
		"set quiet\n",
		"set fallback\n",
		"set unstable\n",
		"set dotenv-load := true\n",
		"set fallback := false\n",
		fmt.Sprintf("set shell := [%s, %s]\n", g.simpleStr(), g.simpleStr()),
		fmt.Sprintf("set tempdir := %s\n", g.simpleStr()),
		fmt.Sprintf("set dotenv-filename := %s\n", g.simpleStr()),
	}
	for i := 0; i < n; i++ {
		b.WriteString(settings[g.rng.IntN(len(settings))])
	}
	return b.String()
}

func (g *stressGen) genAssignment() string {
	prefix := g.oneOf("", "", "", "export ", "eager ", "eager export ")
	return fmt.Sprintf("%s%s := %s\n", prefix, g.id(), g.expr(0))
}

func (g *stressGen) genFuncDef() string {
	name := g.id()
	nParams := 1 + g.rng.IntN(3)
	params := make([]string, nParams)
	for i := range params {
		params[i] = g.id()
	}
	return fmt.Sprintf("%s(%s) := %s\n", name, strings.Join(params, ", "), g.expr(0))
}

func (g *stressGen) genAlias(recipes []string) string {
	if len(recipes) == 0 {
		return ""
	}
	return fmt.Sprintf("alias %s := %s\n", g.id(), recipes[g.rng.IntN(len(recipes))])
}

func (g *stressGen) genRecipe(existing []string) (name string, body string) {
	name = g.id()
	var b strings.Builder

	// Attributes (0-3)
	nAttrs := g.rng.IntN(4)
	for i := 0; i < nAttrs; i++ {
		attr := g.oneOf(
			"[private]",
			"[no-cd]",
			"[linux]",
			"[macos]",
			"[windows]",
			fmt.Sprintf("[group: %s]", g.simpleStr()),
			fmt.Sprintf("[doc(%s)]", g.simpleStr()),
			fmt.Sprintf("[confirm(%s)]", g.simpleStr()),
		)
		b.WriteString(attr + "\n")
	}

	// Quiet
	if g.rng.IntN(5) == 0 {
		b.WriteString("@")
	}

	b.WriteString(name)

	// Params (0-4)
	nParams := g.rng.IntN(5)
	hasVariadic := false
	for i := 0; i < nParams; i++ {
		b.WriteString(" ")
		if !hasVariadic && i == nParams-1 && g.rng.IntN(3) == 0 {
			b.WriteString(g.oneOf("*", "+"))
			hasVariadic = true
		}
		if g.rng.IntN(4) == 0 {
			b.WriteString("$")
		}
		pname := g.id()
		b.WriteString(pname)
		if !hasVariadic && g.rng.IntN(3) == 0 {
			b.WriteString("=" + g.simpleStr())
		}
	}

	b.WriteString(":")

	// Deps (0-3)
	nDeps := g.rng.IntN(4)
	subsequent := false
	for i := 0; i < nDeps && i < len(existing); i++ {
		if !subsequent && i > 0 && g.rng.IntN(3) == 0 {
			b.WriteString(" &&")
			subsequent = true
		}
		dep := existing[g.rng.IntN(len(existing))]
		if g.rng.IntN(3) == 0 {
			fmt.Fprintf(&b, " (%s %s)", dep, g.simpleStr())
		} else {
			b.WriteString(" " + dep)
		}
	}

	b.WriteString("\n")

	// Body (1-5 lines)
	nLines := 1 + g.rng.IntN(5)
	for i := 0; i < nLines; i++ {
		prefix := g.oneOf("", "", "", "@", "-", "@-", "-@")
		switch g.rng.IntN(5) {
		case 0:
			fmt.Fprintf(&b, "    %secho done\n", prefix)
		case 1:
			fmt.Fprintf(&b, "    %secho {{%s}}\n", prefix, g.id())
		case 2:
			fmt.Fprintf(&b, "    %secho {{%s + %s}}\n", prefix, g.simpleStr(), g.simpleStr())
		case 3:
			fmt.Fprintf(&b, "    %secho {{%s(%s)}}\n", prefix,
				g.oneOf("replace", "trim", "uppercase", "lowercase"),
				g.simpleStr())
		case 4:
			fmt.Fprintf(&b, "    %secho {{ if %s == %s { %s } else { %s } }}\n",
				prefix, g.simpleStr(), g.simpleStr(), g.simpleStr(), g.simpleStr())
		default:
		}
	}

	return name, b.String()
}

func (g *stressGen) generate() string {
	var b strings.Builder

	// Shebang (sometimes)
	if g.rng.IntN(5) == 0 {
		b.WriteString("#!/usr/bin/env just --justfile\n\n")
	}

	// Settings
	b.WriteString(g.genSettings(g.rng.IntN(4)))
	b.WriteString("\n")

	// Assignments + function defs
	nAssign := 1 + g.rng.IntN(6)
	for i := 0; i < nAssign; i++ {
		if g.rng.IntN(5) == 0 {
			b.WriteString(g.genFuncDef())
		} else {
			b.WriteString(g.genAssignment())
		}
	}
	b.WriteString("\n")

	// Comments interspersed
	if g.rng.IntN(2) == 0 {
		b.WriteString("# This is a comment\n")
	}

	// Recipes
	var recipes []string
	nRecipes := 2 + g.rng.IntN(8)
	for i := 0; i < nRecipes; i++ {
		if g.rng.IntN(3) == 0 {
			fmt.Fprintf(&b, "# Recipe %d docs\n", i)
		}
		name, recipe := g.genRecipe(recipes)
		recipes = append(recipes, name)
		b.WriteString(recipe)
		b.WriteString("\n")
	}

	// Aliases
	nAliases := g.rng.IntN(3)
	for i := 0; i < nAliases; i++ {
		b.WriteString(g.genAlias(recipes))
	}

	// Unexports (sometimes)
	if g.rng.IntN(4) == 0 {
		b.WriteString("unexport " + g.id() + "\n")
	}

	return b.String()
}

func TestStressGenerated(t *testing.T) {
	numFiles := int64(2000)
	if testing.Short() {
		numFiles = 200
	}

	for seed := int64(0); seed < numFiles; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			g := newStressGen(seed)
			content := g.generate()

			jf, err := Parse([]byte(content))
			if err != nil {
				t.Fatalf("seed %d: Parse failed: %v\n\nContent:\n%s", seed, err, content)
			}

			// Validate shouldn't panic
			_ = jf.Validate()

			// Basic sanity checks
			if len(jf.Recipes) == 0 {
				t.Error("expected at least one recipe")
			}
		})
	}
}

// TestStressSpecificPatterns tests patterns that are syntactically tricky
// and likely to expose parser edge cases.
func TestStressSpecificPatterns(t *testing.T) {
	patterns := []struct {
		name    string
		content string
	}{
		// Deeply nested conditionals
		{"nested_if_3", `v := if "a" == "b" { if "c" == "d" { if "e" == "f" { "g" } else { "h" } } else { "i" } } else { "j" }` + "\n"},

		// Chained operators
		{"chain_concat_10", `v := "a" + "b" + "c" + "d" + "e" + "f" + "g" + "h" + "i" + "j"` + "\n"},
		{"chain_path_10", `v := "a" / "b" / "c" / "d" / "e" / "f" / "g" / "h" / "i" / "j"` + "\n"},
		{"chain_and", `v := "a" && "b" && "c" && "d"` + "\n"},
		{"chain_or", `v := "a" || "b" || "c" || "d"` + "\n"},
		{"mixed_ops", `v := "a" + "b" / "c" + "d"` + "\n"},

		// Many params
		{"many_params", "r a b c d e f g h:\n    echo done\n"},
		{"many_deps", "a:\n    echo a\nb:\n    echo b\nc:\n    echo c\nd: a b c\n    echo d\n"},
		{"many_subsequent_deps", "a:\n    echo a\nb:\n    echo b\nc:\n    echo c\nd: a && b c\n    echo d\n"},

		// All attribute forms on one recipe
		{"all_attrs", "[private]\n[no-cd]\n[linux]\n[group: 'test']\n[doc('my recipe')]\n[confirm('sure?')]\nbuild:\n    echo done\n"},

		// Multiple attributes on one line
		{"multi_attr_line", "[private, no-cd, linux]\nbuild:\n    echo done\n"},

		// Empty body
		{"empty_body", "build:\n"},
		{"empty_body_with_deps", "a:\n    echo a\nb: a\n"},

		// Recipe with only shebang
		{"shebang_only", "build:\n    #!/bin/bash\n"},

		// Many settings
		{"all_bool_settings", "set dotenv-load\nset export\nset positional-arguments\nset quiet\nset fallback\nset unstable\n"},

		// Multiline function def with nested if
		{"funcdef_multiline_nested", "f(a, b) := if a == b {\n  if a == 'x' {\n    'xx'\n  } else {\n    'ab'\n  }\n} else {\n  'diff'\n}\n"},

		// String with every escape
		{"all_escapes", `v := "tab\there\nnewline\rcarriage\"\u{0041}"` + "\n"},

		// Indented strings
		{"indented_quoted", "v := \"\"\"\n  hello\n  world\n\"\"\"\n"},
		{"indented_raw", "v := '''\n  hello\n  world\n'''\n"},
		{"indented_backtick", "v := ```\n  echo hello\n  echo world\n```\n"},

		// Format and shell-expanded strings
		{"fstring_quoted", `v := f"hello {name}"` + "\n"},
		{"fstring_raw", "v := f'hello {name}'\n"},
		{"xstring_quoted", `v := x"$HOME/bin"` + "\n"},
		{"xstring_raw", "v := x'$HOME/bin'\n"},

		// Leading slash
		{"leading_slash", `v := / "usr" / "local"` + "\n"},
		{"leading_slash_complex", `v := / "usr" / "local" / "bin"` + "\n"},

		// Brace escape
		{"brace_escape", "build:\n    echo '{{{{literal}}'\n"},

		// Recipe named with keyword
		{"recipe_named_import", "import:\n    echo done\n"},

		// Comment before recipe with attributes
		{"comment_attr_recipe", "# docs\n[private]\nbuild:\n    echo done\n"},

		// Multiline string in dep arg
		{"multiline_dep_arg", "build x:\n    echo done\nfoo: (build 'multi\nline')\n    echo foo\n"},

		// Multiline string in interpolation
		{"multiline_interp", "build:\n    echo {{'multi\nline'}}\n"},

		// Multiline string in conditional
		{"multiline_cond", "v := if 'a' == 'b' { 'yes\nmulti' } else { 'no\nmulti' }\n"},

		// Eager export
		{"eager_export", "eager export TOKEN := `echo hi`\n"},
		{"export_eager", "export eager TOKEN := `echo hi`\n"},

		// Many recipes with cross-deps
		{"cross_deps", "a:\n    echo a\nb: a\n    echo b\nc: a b\n    echo c\nd: a b c\n    echo d\ne: a b c d\n    echo e\n"},

		// Recipe with complex interpolation
		{"complex_interp", "build:\n    echo {{ replace(arch() + \"-\" + os(), \"-\", \"_\") }}\n"},

		// Backtick in expression
		{"backtick_expr", "v := `echo hello` + \" \" + `echo world`\n"},

		// Paren expression
		{"paren_expr", `v := ("a" + "b") + ("c" + "d")` + "\n"},

		// Optional import and mod
		{"optional_both", "import? 'maybe.just'\nmod? maybe\nbuild:\n    echo done\n"},

		// Blank lines everywhere
		{"blank_lines", "\n\n\nv := 'a'\n\n\nbuild:\n    echo done\n\n\n"},

		// Unicode escape
		{"unicode", `v := "\u{1F600}\u{0041}\u{00E9}"` + "\n"},

		// Line continuation
		{"line_cont", "v := \"a\" + \\\n  \"b\" + \\\n  \"c\"\n"},

		// Setting with list trailing comma
		{"list_trailing_comma", "set shell := ['bash', '-cu',]\n"},
	}

	for _, p := range patterns {
		t.Run(p.name, func(t *testing.T) {
			jf, err := Parse([]byte(p.content))
			if err != nil {
				t.Fatalf("Parse failed: %v\n\nContent:\n%s", err, p.content)
			}
			_ = jf.Validate()
		})
	}
}
