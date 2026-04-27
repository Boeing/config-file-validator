package gojust

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// gen is a justfile generator that produces random but grammatically
// plausible justfiles by combining productions from the just grammar.
type gen struct {
	rng   *rand.Rand
	depth int
}

func newGen(seed int64) *gen {
	return &gen{rng: rand.New(rand.NewPCG(uint64(seed), uint64(seed)))}
}

func (g *gen) pick(choices ...string) string {
	return choices[g.rng.IntN(len(choices))]
}

func (g *gen) ident() string {
	prefixes := []string{"foo", "bar", "baz", "build", "test", "deploy", "lint", "check", "run", "clean", "install", "update", "my-task", "do_thing", "_private"}
	suffix := fmt.Sprintf("_%d", g.rng.IntN(100))
	return prefixes[g.rng.IntN(len(prefixes))] + suffix
}

func (g *gen) stringLit() string {
	switch g.rng.IntN(4) {
	case 0:
		return fmt.Sprintf(`"hello_%d"`, g.rng.IntN(100))
	case 1:
		return fmt.Sprintf(`'raw_%d'`, g.rng.IntN(100))
	case 2:
		return `"with\nescaped"`
	default:
		// Multi-line raw string — valid in assignments, dep args, interpolations
		return "'multi\nline\nraw'"
	}
}

func (g *gen) expr() string {
	g.depth++
	defer func() { g.depth-- }()
	if g.depth > 3 {
		return g.stringLit()
	}
	switch g.rng.IntN(8) {
	case 0:
		return g.stringLit()
	case 1:
		return g.ident()
	case 2:
		return g.expr() + " + " + g.expr()
	case 3:
		return g.expr() + " / " + g.expr()
	case 4:
		return g.pick("arch", "os", "os_family", "num_cpus", "justfile_directory") + "()"
	case 5:
		return fmt.Sprintf(`env_var_or_default("%s", %s)`, g.ident(), g.stringLit())
	case 6:
		return fmt.Sprintf(`replace(%s, ".", "-")`, g.stringLit())
	case 7:
		return fmt.Sprintf(`if %s == %s { %s } else { %s }`,
			g.stringLit(), g.stringLit(), g.stringLit(), g.stringLit())
	}
	return g.stringLit()
}

func (g *gen) assignment() string {
	prefix := g.pick("", "export ", "eager ", "eager export ")
	return fmt.Sprintf("%s%s := %s\n", prefix, g.ident(), g.expr())
}

func (g *gen) setting() string {
	switch g.rng.IntN(4) {
	case 0:
		return fmt.Sprintf("set %s\n", g.pick("dotenv-load", "export", "positional-arguments", "quiet"))
	case 1:
		return fmt.Sprintf("set %s := %s\n", g.pick("dotenv-load", "fallback"), g.pick("true", "false"))
	case 2:
		return fmt.Sprintf("set shell := [%s, %s]\n", g.stringLit(), g.stringLit())
	default:
		return fmt.Sprintf("set tempdir := %s\n", g.stringLit())
	}
}

func (g *gen) param() string {
	variadic := g.pick("", "", "", "*", "+")
	export := g.pick("", "", "$")
	name := g.ident()
	dflt := ""
	if g.rng.IntN(3) == 0 && variadic == "" {
		dflt = "=" + g.stringLit()
	}
	return variadic + export + name + dflt
}

func (g *gen) dep(recipes []string) string {
	if len(recipes) == 0 {
		return ""
	}
	name := recipes[g.rng.IntN(len(recipes))]
	if g.rng.IntN(3) == 0 {
		return fmt.Sprintf("(%s %s)", name, g.stringLit())
	}
	return name
}

func (g *gen) singleLineStringLit() string {
	switch g.rng.IntN(2) {
	case 0:
		return fmt.Sprintf(`"hello_%d"`, g.rng.IntN(100))
	default:
		return fmt.Sprintf(`'raw_%d'`, g.rng.IntN(100))
	}
}

func (g *gen) recipeLine() string {
	prefix := g.pick("", "", "@", "-", "@-", "-@")
	switch g.rng.IntN(4) {
	case 0:
		// Shell text — use single-line strings (shell quotes, not justfile strings)
		return fmt.Sprintf("    %secho %s\n", prefix, g.singleLineStringLit())
	case 1:
		// Interpolation — justfile expressions, multi-line strings OK
		return fmt.Sprintf("    %secho {{%s}}\n", prefix, g.ident())
	case 2:
		return fmt.Sprintf("    %secho {{%s + %s}}\n", prefix, g.stringLit(), g.stringLit())
	default:
		return fmt.Sprintf("    %secho done\n", prefix)
	}
}

func (g *gen) attribute() string {
	switch g.rng.IntN(6) {
	case 0:
		return "[private]\n"
	case 1:
		return fmt.Sprintf("[group: %s]\n", g.stringLit())
	case 2:
		return "[no-cd]\n"
	case 3:
		return fmt.Sprintf("[confirm(%s)]\n", g.stringLit())
	case 4:
		return "[linux]\n"
	default:
		return fmt.Sprintf("[doc(%s)]\n", g.stringLit())
	}
}

func (g *gen) recipe(existingRecipes []string) (name string, body string) {
	name = g.ident()
	var b strings.Builder

	// Attributes
	if g.rng.IntN(3) == 0 {
		b.WriteString(g.attribute())
	}

	// Quiet prefix
	if g.rng.IntN(4) == 0 {
		b.WriteString("@")
	}

	b.WriteString(name)

	// Params
	nParams := g.rng.IntN(3)
	for i := 0; i < nParams; i++ {
		b.WriteString(" " + g.param())
	}

	b.WriteString(":")

	// Deps
	nDeps := g.rng.IntN(3)
	for i := 0; i < nDeps; i++ {
		d := g.dep(existingRecipes)
		if d != "" {
			b.WriteString(" " + d)
		}
	}

	// Subsequent deps
	if g.rng.IntN(4) == 0 && len(existingRecipes) > 0 {
		b.WriteString(" && " + g.dep(existingRecipes))
	}

	b.WriteString("\n")

	// Body
	nLines := 1 + g.rng.IntN(3)
	for i := 0; i < nLines; i++ {
		b.WriteString(g.recipeLine())
	}

	return name, b.String()
}

func (g *gen) justfile() string {
	var b strings.Builder

	// Settings
	nSettings := g.rng.IntN(4)
	for i := 0; i < nSettings; i++ {
		b.WriteString(g.setting())
	}
	if nSettings > 0 {
		b.WriteString("\n")
	}

	// Assignments
	nAssignments := 1 + g.rng.IntN(5)
	for i := 0; i < nAssignments; i++ {
		b.WriteString(g.assignment())
	}
	b.WriteString("\n")

	// Recipes
	var recipeNames []string
	nRecipes := 2 + g.rng.IntN(6)
	for i := 0; i < nRecipes; i++ {
		name, recipe := g.recipe(recipeNames)
		recipeNames = append(recipeNames, name)
		b.WriteString(recipe)
		b.WriteString("\n")
	}

	// Aliases
	if len(recipeNames) > 1 && g.rng.IntN(2) == 0 {
		target := recipeNames[g.rng.IntN(len(recipeNames))]
		fmt.Fprintf(&b, "alias a_%d := %s\n", g.rng.IntN(100), target)
	}

	return b.String()
}

func TestGeneratedJustfiles(t *testing.T) {
	const numFiles = 1000

	for seed := int64(0); seed < numFiles; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			g := newGen(seed)
			content := g.justfile()

			jf, err := Parse([]byte(content))
			if err != nil {
				t.Fatalf("seed %d: Parse failed: %v\n\nContent:\n%s", seed, err, content)
			}

			// Also run validation — shouldn't panic
			_ = jf.Validate()
		})
	}
}

func TestGeneratedEdgeCases(t *testing.T) {
	// Specific patterns that combine grammar features in unusual ways
	cases := []struct {
		name    string
		content string
	}{
		{"empty recipe", "build:\n"},
		{"recipe no newline at end", "build:\n    echo done"},
		{"multiple blank lines", "\n\n\nbuild:\n    echo done\n\n\n"},
		{"comment between attrs", "[private]\n# comment\n[no-cd]\nbuild:\n    echo done\n"},
		{"all string types in assignments", `
a := "quoted"
b := 'raw'
c := ` + "`backtick`" + `
d := f"format"
e := f'format_raw'
f := x"shell"
g := x'shell_raw'
`},
		{"deeply nested expression", `val := "a" + "b" + "c" + "d" + "e" + "f" + "g"` + "\n"},
		{"deeply nested path", `val := "a" / "b" / "c" / "d" / "e"` + "\n"},
		{"conditional in conditional", `val := if "a" == "b" { if "c" == "d" { "e" } else { "f" } } else { "g" }` + "\n"},
		{"recipe with all param types", "build target mode=\"debug\" $ENV +rest:\n    echo done\n"},
		{"subsequent deps with args", "all: build && (test \"integration\") deploy\n    echo done\nbuild:\n    echo build\ntest x:\n    echo test\ndeploy:\n    echo deploy\n"},
		{"shebang recipe", "build:\n    #!/usr/bin/env bash\n    set -euo pipefail\n    echo done\n"},
		{"multiline conditional assignment", "val := if \"a\" == \"b\" {\n  \"yes\"\n} else {\n  \"no\"\n}\n"},
		{"function def multiline", "myfn(a, b) := if a == b {\n  'same'\n} else {\n  'diff'\n}\n"},
		{"recipe named import", "import:\n    echo importing\n"},
		{"eager export", "eager export TOKEN := `echo secret`\n"},
		{"setting with keyword name", "set export\nset quiet\n"},
		{"attribute with multiple args", "[env(name=\"FOO\", value=\"bar\")]\nbuild:\n    echo done\n"},
		{"brace escape", "build:\n    echo '{{{{literal braces}}'\n"},
		{"recipe with empty lines in body", "build:\n    echo start\n\n    echo end\n"},
		{"absolute path expression", "val := / \"usr\" / \"local\" / \"bin\"\n"},
		{"backtick in expression", "val := `echo hello` + \" world\"\n"},
		{"complex deps", "all: (build \"release\") && (test \"unit\") (deploy \"prod\")\n    echo done\nbuild x:\n    echo build\ntest x:\n    echo test\ndeploy x:\n    echo deploy\n"},
		{"optional import and mod", "import? 'maybe.just'\nmod? optional\nbuild:\n    echo done\n"},
		{"unexport", "export FOO := \"bar\"\nunexport FOO\nbuild:\n    echo done\n"},
		{"private assignment", "[private]\n_internal := \"secret\"\nbuild:\n    echo done\n"},
		{"string operators", "a := 'hello' && 'world'\nb := '' || 'fallback'\n"},
		{"regex in conditional", "val := if arch() =~ \"x86\" { \"intel\" } else { \"other\" }\n"},
		{"many settings", "set dotenv-load\nset export\nset positional-arguments\nset quiet\nset fallback\n"},
		{"long recipe body", "build:\n    echo line1\n    echo line2\n    echo line3\n    echo line4\n    echo line5\n    echo line6\n    echo line7\n    echo line8\n    echo line9\n    echo line10\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jf, err := Parse([]byte(tc.content))
			if err != nil {
				t.Fatalf("Parse failed: %v\n\nContent:\n%s", err, tc.content)
			}
			_ = jf.Validate()
		})
	}
}
