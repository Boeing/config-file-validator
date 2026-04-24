package gojust

import "fmt"

// Validate performs semantic analysis on a parsed justfile and returns
// any diagnostics found. It checks for duplicate definitions, undefined
// references, and invalid configurations.
func (jf *Justfile) Validate() []Diagnostic {
	a := &analyzer{
		file:      jf.File,
		recipes:   make(map[string]Position),
		variables: make(map[string]Position),
		aliases:   make(map[string]Position),
	}
	a.analyze(jf)
	return a.diagnostics
}

type analyzer struct {
	file                    string
	diagnostics             []Diagnostic
	recipes                 map[string]Position
	variables               map[string]Position
	aliases                 map[string]Position
	allowDuplicateRecipes   bool
	allowDuplicateVariables bool
}

func (a *analyzer) analyze(jf *Justfile) {
	for _, s := range jf.Settings {
		if s.Name == "allow-duplicate-recipes" && s.Value.Bool != nil && *s.Value.Bool {
			a.allowDuplicateRecipes = true
		}
		if s.Name == "allow-duplicate-variables" && s.Value.Bool != nil && *s.Value.Bool {
			a.allowDuplicateVariables = true
		}
	}

	a.collectDefinitions(jf)
	a.checkRecipes(jf)
	a.checkAliases(jf)
	a.checkSettings(jf)
	a.checkVariableRefs(jf)
	a.checkExportUnexportConflicts(jf)
}

func (a *analyzer) collectDefinitions(jf *Justfile) {
	for _, r := range jf.Recipes {
		if prev, ok := a.recipes[r.Name]; ok && !a.allowDuplicateRecipes {
			a.error(r.Pos, "recipe '%s' is defined multiple times (first defined at %s)", r.Name, prev)
		}
		a.recipes[r.Name] = r.Pos
	}

	for _, v := range jf.Assignments {
		if prev, ok := a.variables[v.Name]; ok && !a.allowDuplicateVariables {
			a.error(v.Pos, "variable '%s' is defined multiple times (first defined at %s)", v.Name, prev)
		}
		a.variables[v.Name] = v.Pos
	}

	for _, al := range jf.Aliases {
		if prev, ok := a.aliases[al.Name]; ok {
			a.error(al.Pos, "alias '%s' is defined multiple times (first defined at %s)", al.Name, prev)
		}
		a.aliases[al.Name] = al.Pos
	}

	// Collect from resolved imports
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			a.collectDefinitions(imp.Justfile)
		}
	}
}

func (a *analyzer) checkRecipes(jf *Justfile) {
	for _, r := range jf.Recipes {
		a.checkDependencies(r)
		a.checkParameters(r)
	}
	a.checkCircularDeps(jf)
}

func (a *analyzer) checkDependencies(r *Recipe) {
	for _, dep := range r.Dependencies {
		if _, ok := a.recipes[dep.Name]; !ok {
			a.error(dep.Pos, "recipe '%s' depends on undefined recipe '%s'", r.Name, dep.Name)
		}
	}
}

func (a *analyzer) checkParameters(r *Recipe) {
	seen := make(map[string]bool)
	foundVariadic := false
	for _, p := range r.Parameters {
		if seen[p.Name] {
			a.error(p.Pos, "recipe '%s' has duplicate parameter '%s'", r.Name, p.Name)
		}
		seen[p.Name] = true

		if foundVariadic {
			a.error(p.Pos, "recipe '%s' has parameters after variadic parameter", r.Name)
		}
		if p.Variadic != "" {
			foundVariadic = true
		}
	}
}

func (a *analyzer) checkCircularDeps(jf *Justfile) {
	depGraph := make(map[string][]string)
	a.buildDepGraph(jf, depGraph)

	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var visit func(name string) bool
	visit = func(name string) bool {
		if inStack[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visited[name] = true
		inStack[name] = true
		for _, dep := range depGraph[name] {
			if visit(dep) {
				if pos, ok := a.recipes[name]; ok {
					a.error(pos, "recipe '%s' has a circular dependency", name)
				}
				return true
			}
		}
		inStack[name] = false
		return false
	}

	for name := range depGraph {
		visit(name)
	}
}

// buildDepGraph collects recipe dependencies including from imports.
func (a *analyzer) buildDepGraph(jf *Justfile, graph map[string][]string) {
	for _, r := range jf.Recipes {
		for _, dep := range r.Dependencies {
			graph[r.Name] = append(graph[r.Name], dep.Name)
		}
	}
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			a.buildDepGraph(imp.Justfile, graph)
		}
	}
}

func (a *analyzer) checkAliases(jf *Justfile) {
	for _, al := range jf.Aliases {
		if _, ok := a.recipes[al.Target]; !ok {
			a.error(al.Pos, "alias '%s' targets undefined recipe '%s'", al.Name, al.Target)
		}
	}
}

var knownSettings = func() map[string]bool {
	m := make(map[string]bool, len(knownSettingsList))
	for _, s := range knownSettingsList {
		m[s.Name] = true
	}
	return m
}()

func (a *analyzer) checkSettings(jf *Justfile) {
	seen := make(map[string]bool)
	for _, s := range jf.Settings {
		if !knownSettings[s.Name] {
			a.warn(s.Pos, "unknown setting '%s'", s.Name)
		}
		if seen[s.Name] {
			a.error(s.Pos, "setting '%s' is set multiple times", s.Name)
		}
		seen[s.Name] = true
	}
}

// walkExpr calls fn for every sub-expression in expr, depth-first.
func walkExpr(expr Expression, fn func(Expression)) {
	if expr == nil {
		return
	}
	fn(expr)
	switch e := expr.(type) {
	case *Concatenation:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *PathJoin:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *LogicalOp:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *Comparison:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *Conditional:
		walkExpr(e.Condition.Left, fn)
		walkExpr(e.Condition.Right, fn)
		walkExpr(e.Then, fn)
		walkExpr(e.Otherwise, fn)
	case *FunctionCall:
		for _, arg := range e.Arguments {
			walkExpr(arg, fn)
		}
	case *ParenExpr:
		walkExpr(e.Inner, fn)
	default:
		// leaf nodes: StringLiteral, Variable, BacktickExpr
	}
}

// builtinFunctions is the set of functions built into just.
var builtinFunctions = map[string]bool{
	"absolute_path": true, "arch": true, "blake3": true, "blake3_file": true,
	"cache_directory": true, "canonicalize": true, "capitalize": true,
	"choose": true, "clean": true, "config_directory": true,
	"config_local_directory": true, "data_directory": true,
	"data_local_directory": true, "datetime": true, "datetime_utc": true,
	"encode_uri_component": true, "env": true, "env_var": true,
	"env_var_or_default": true, "error": true, "executable_directory": true,
	"extension": true, "file_name": true, "file_stem": true,
	"home_directory": true, "invocation_directory": true,
	"invocation_directory_native": true, "is_dependency": true,
	"join": true, "just_executable": true, "just_pid": true,
	"justfile": true, "justfile_directory": true, "kebabcase": true,
	"lowercamelcase": true, "lowercase": true, "module_directory": true,
	"module_file": true, "num_cpus": true, "os": true, "os_family": true,
	"parent_directory": true, "path_exists": true, "prepend": true,
	"quote": true, "read": true, "read_to_string": true, "replace": true,
	"replace_regex": true, "require": true, "semver_matches": true,
	"sha256": true, "sha256_file": true, "shell": true, "shoutykebabcase": true,
	"shoutysnakecase": true, "snakecase": true, "source_directory": true,
	"source_file": true, "style": true, "titlecase": true, "trim": true,
	"trim_end": true, "trim_end_match": true, "trim_end_matches": true,
	"trim_start": true, "trim_start_match": true, "trim_start_matches": true,
	"uppercamelcase": true, "uppercase": true, "uuid": true, "which": true,
	"without_extension": true,
}

func (a *analyzer) checkVariableRefs(jf *Justfile) {
	// Check variable references in assignment expressions
	for _, assign := range jf.Assignments {
		// Function definition parameters are local scope
		locals := make(map[string]bool)
		for _, p := range assign.Parameters {
			locals[p] = true
		}
		a.checkExprVarRefs(assign.Value, locals)
	}

	// Check variable references in recipe bodies and dependency args
	for _, r := range jf.Recipes {
		locals := make(map[string]bool)
		for _, p := range r.Parameters {
			locals[p.Name] = true
		}

		for _, dep := range r.Dependencies {
			for _, arg := range dep.Arguments {
				a.checkExprVarRefs(arg, locals)
			}
		}

		for _, line := range r.Body {
			for _, frag := range line.Fragments {
				if interp, ok := frag.(*InterpolationFragment); ok {
					a.checkExprVarRefs(interp.Expression, locals)
				}
			}
		}
	}
}

func (a *analyzer) checkExprVarRefs(expr Expression, locals map[string]bool) {
	walkExpr(expr, func(e Expression) {
		switch v := e.(type) {
		case *Variable:
			if len(locals) > 0 && locals[v.Name] {
				return
			}
			if _, ok := a.variables[v.Name]; ok {
				return
			}
			a.error(v.Pos, "undefined variable '%s'", v.Name)
		case *FunctionCall:
			if !builtinFunctions[v.Name] {
				if _, ok := a.variables[v.Name]; !ok {
					a.error(v.Pos, "undefined function '%s'", v.Name)
				}
			}
		default:
		}
	})
}

func (a *analyzer) checkExportUnexportConflicts(jf *Justfile) {
	exported := make(map[string]Position)
	a.collectExports(jf, exported)

	unexported := make(map[string]Position)
	a.collectUnexports(jf, unexported)

	for name, unexportPos := range unexported {
		if exportPos, ok := exported[name]; ok {
			a.error(unexportPos, "variable '%s' is both exported (at %s) and unexported", name, exportPos)
		}
	}
}

func (a *analyzer) collectExports(jf *Justfile, exported map[string]Position) {
	for _, v := range jf.Assignments {
		if v.Export {
			exported[v.Name] = v.Pos
		}
	}
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			a.collectExports(imp.Justfile, exported)
		}
	}
}

func (a *analyzer) collectUnexports(jf *Justfile, unexported map[string]Position) {
	for _, u := range jf.Unexports {
		unexported[u.Name] = u.Pos
	}
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			a.collectUnexports(imp.Justfile, unexported)
		}
	}
}

func (a *analyzer) error(pos Position, format string, args ...any) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Pos:      pos,
		Severity: SeverityError,
		Message:  fmt.Sprintf(format, args...),
		File:     a.file,
	})
}

func (a *analyzer) warn(pos Position, format string, args ...any) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Pos:      pos,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		File:     a.file,
	})
}
