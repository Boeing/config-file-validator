//go:build justdump

package gojust

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// toJustDump converts a parsed Justfile AST into the same JSON structure
// produced by `just --dump --dump-format json` (pinned to v1.49.0).
// Used internally for compatibility testing.
func toJustDump(jf *Justfile, namePrefix string) map[string]interface{} {
	recipes := collectRecipes(jf, namePrefix)
	assignments := collectAssignments(jf)
	aliases := collectAliases(jf)
	unexports := collectUnexports(jf)

	var first interface{} = nil
	if len(jf.Recipes) > 0 {
		first = jf.Recipes[0].Name
	}
	if first == nil {
		for _, imp := range jf.Imports {
			if imp.Justfile != nil && len(imp.Justfile.Recipes) > 0 {
				first = imp.Justfile.Recipes[0].Name
				break
			}
		}
	}

	modules := make(map[string]interface{})
	for _, mod := range jf.Modules {
		if mod.Justfile != nil {
			modules[mod.Name] = toJustDump(mod.Justfile, namePrefix+mod.Name+"::")
		}
	}

	return map[string]interface{}{
		"aliases":     aliases,
		"assignments": assignments,
		"first":       first,
		"doc":         nil,
		"groups":      []interface{}{},
		"modules":     modules,
		"recipes":     recipes,
		"settings":    buildSettings(jf),
		"source":      jf.File,
		"unexports":   unexports,
		"warnings":    []interface{}{},
	}
}

func collectRecipes(jf *Justfile, namePrefix string) map[string]interface{} {
	recipes := make(map[string]interface{})
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			for k, v := range collectRecipes(imp.Justfile, namePrefix) {
				recipes[k] = v
			}
		}
	}
	for _, r := range jf.Recipes {
		recipes[r.Name] = dumpRecipe(r, namePrefix)
	}
	return recipes
}

func collectAssignments(jf *Justfile) map[string]interface{} {
	assignments := make(map[string]interface{})
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			for k, v := range collectAssignments(imp.Justfile) {
				assignments[k] = v
			}
		}
	}
	for _, a := range jf.Assignments {
		// Function definitions (with parameters) are not included in just's dump
		if len(a.Parameters) > 0 {
			continue
		}
		assignments[a.Name] = dumpAssignment(a)
	}
	return assignments
}

func collectAliases(jf *Justfile) map[string]interface{} {
	aliases := make(map[string]interface{})
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			for k, v := range collectAliases(imp.Justfile) {
				aliases[k] = v
			}
		}
	}
	for _, a := range jf.Aliases {
		aliases[a.Name] = dumpAlias(a)
	}
	return aliases
}

func collectUnexports(jf *Justfile) []interface{} {
	var unexports []interface{}
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			unexports = append(unexports, collectUnexports(imp.Justfile)...)
		}
	}
	for _, u := range jf.Unexports {
		unexports = append(unexports, u.Name)
	}
	if unexports == nil {
		return []interface{}{}
	}
	return unexports
}

func dumpRecipe(r *Recipe, namePrefix string) map[string]interface{} {
	params := make([]interface{}, 0, len(r.Parameters))
	for _, p := range r.Parameters {
		params = append(params, dumpParameter(p))
	}

	deps := make([]interface{}, 0, len(r.Dependencies))
	priors := 0
	for _, d := range r.Dependencies {
		dep := map[string]interface{}{
			"arguments": dumpDepArgs(d.Arguments),
			"recipe":    d.Name,
		}
		deps = append(deps, dep)
		if !d.Subsequent {
			priors++
		}
	}

	body := make([]interface{}, 0, len(r.Body))
	for _, line := range r.Body {
		body = append(body, dumpRecipeLine(line))
	}

	attrs := dumpAttributes(r.Attributes)

	var doc interface{} = nil
	if r.Comment != "" {
		doc = r.Comment
	}
	isPrivate := len(r.Name) > 0 && r.Name[0] == '_'
	isShebang := r.Shebang != ""
	for _, attr := range r.Attributes {
		switch attr.Name {
		case "doc":
			if len(attr.Arguments) > 0 {
				doc = attr.Arguments[0].Value
			}
		case "private":
			isPrivate = true
		case "script":
			isShebang = true
		}
	}

	return map[string]interface{}{
		"attributes":   attrs,
		"body":         body,
		"dependencies": deps,
		"doc":          doc,
		"name":         r.Name,
		"parameters":   params,
		"priors":       priors,
		"private":      isPrivate,
		"quiet":        r.Quiet,
		"namepath":     namePrefix + r.Name,
		"shebang":      isShebang,
	}
}

func dumpParameter(p *Parameter) map[string]interface{} {
	kind := "singular"
	switch p.Variadic {
	case "*":
		kind = "star"
	case "+":
		kind = "plus"
	}

	var def interface{} = nil
	if p.Default != nil {
		def = dumpExpression(p.Default)
	}

	return map[string]interface{}{
		"default": def,
		"export":  p.Export,
		"help":    nil,
		"kind":    kind,
		"long":    nil,
		"name":    p.Name,
		"pattern": nil,
		"short":   nil,
		"value":   nil,
	}
}

func dumpDepArgs(args []Expression) []interface{} {
	result := make([]interface{}, 0, len(args))
	for _, a := range args {
		result = append(result, dumpExpression(a))
	}
	return result
}

func dumpRecipeLine(line *RecipeLine) []interface{} {
	var parts []interface{}

	for i, frag := range line.Fragments {
		switch f := frag.(type) {
		case *TextFragment:
			text := reescapeBraces(f.Value)
			if i == 0 {
				text = applyLinePrefix(line, text)
			}
			parts = append(parts, text)
		case *InterpolationFragment:
			parts = append(parts, []interface{}{dumpExpression(f.Expression)})
		}
	}

	if len(parts) == 0 {
		prefix := applyLinePrefix(line, "")
		if prefix != "" {
			parts = append(parts, prefix)
		}
	}

	return parts
}

// reescapeBraces converts {{ back to {{{{ in recipe body text,
// matching just --dump's output which preserves the escape syntax.
func reescapeBraces(s string) string {
	return strings.ReplaceAll(s, "{{", "{{{{")
}

func applyLinePrefix(line *RecipeLine, text string) string {
	prefix := ""
	if line.Quiet && line.NoError {
		prefix = line.PrefixOrder
		if prefix == "" {
			prefix = "@-" // fallback
		}
	} else if line.Quiet {
		prefix = "@"
	} else if line.NoError {
		prefix = "-"
	}
	return prefix + text
}

func dumpExpression(expr Expression) interface{} {
	switch e := expr.(type) {
	case *StringLiteral:
		return e.Value
	case *Variable:
		return []interface{}{"variable", e.Name}
	case *Concatenation:
		return []interface{}{"concatenate", dumpExpression(e.Left), dumpExpression(e.Right)}
	case *PathJoin:
		left := dumpExpression(e.Left)
		// Leading / produces a PathJoin with empty string left — just encodes as null
		if s, ok := left.(string); ok && s == "" {
			left = nil
		}
		return []interface{}{"join", left, dumpExpression(e.Right)}
	case *FunctionCall:
		parts := []interface{}{"call", e.Name}
		for _, arg := range e.Arguments {
			parts = append(parts, dumpExpression(arg))
		}
		return parts
	case *BacktickExpr:
		return []interface{}{"evaluate", e.Command}
	case *Conditional:
		cond := []interface{}{
			e.Condition.Operator,
			dumpExpression(e.Condition.Left),
			dumpExpression(e.Condition.Right),
		}
		return []interface{}{"if", cond, dumpExpression(e.Then), dumpExpression(e.Otherwise)}
	case *ParenExpr:
		return dumpExpression(e.Inner)
	case *LogicalOp:
		op := e.Operator
		if op == "&&" {
			op = "and"
		} else if op == "||" {
			op = "or"
		}
		return []interface{}{op, dumpExpression(e.Left), dumpExpression(e.Right)}
	case *Comparison:
		return []interface{}{e.Operator, dumpExpression(e.Left), dumpExpression(e.Right)}
	default:
		panic(fmt.Sprintf("gojust: unhandled expression type %T in dumpExpression", expr))
	}
}

func dumpAlias(a *Alias) map[string]interface{} {
	return map[string]interface{}{
		"attributes": dumpAttributes(a.Attributes),
		"name":       a.Name,
		"target":     a.Target,
	}
}

func dumpAssignment(a *Assignment) map[string]interface{} {
	isPrivate := a.Private || (len(a.Name) > 0 && a.Name[0] == '_')
	return map[string]interface{}{
		"eager":   a.Eager,
		"export":  a.Export,
		"name":    a.Name,
		"private": isPrivate,
		"value":   dumpExpression(a.Value),
	}
}

func dumpAttributes(attrs []*Attribute) []interface{} {
	// just sorts attributes alphabetically by name
	sorted := make([]*Attribute, len(attrs))
	copy(sorted, attrs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	result := make([]interface{}, 0, len(sorted))
	for _, attr := range sorted {
		result = append(result, dumpAttribute(attr))
	}
	return result
}

// knownParameterizedAttributes lists attributes that just always encodes as
// objects with a value (null if no argument was provided).
var knownParameterizedAttributes = map[string]bool{
	"confirm": true,
	"group":   true,
	"doc":     true,
	"script":  true,
}

func dumpAttribute(attr *Attribute) interface{} {
	if len(attr.Arguments) == 0 {
		if knownParameterizedAttributes[attr.Name] {
			return map[string]interface{}{attr.Name: nil}
		}
		return attr.Name
	}
	// [script('bash')] → {"script": {"command": "bash", "arguments": []}}
	if attr.Name == "script" {
		if len(attr.Arguments) >= 1 && attr.Arguments[0].Key == "" {
			args := make([]interface{}, 0)
			for _, a := range attr.Arguments[1:] {
				args = append(args, a.Value)
			}
			return map[string]interface{}{"script": map[string]interface{}{
				"command":   attr.Arguments[0].Value,
				"arguments": args,
			}}
		}
	}
	if len(attr.Arguments) == 1 && attr.Arguments[0].Key == "" {
		return map[string]interface{}{attr.Name: attr.Arguments[0].Value}
	}
	if len(attr.Arguments) == 1 && attr.Arguments[0].Key != "" {
		return map[string]interface{}{attr.Name: map[string]interface{}{attr.Arguments[0].Key: attr.Arguments[0].Value}}
	}
	args := make([]interface{}, len(attr.Arguments))
	for i, a := range attr.Arguments {
		if a.Key != "" {
			args[i] = map[string]interface{}{a.Key: a.Value}
		} else {
			args[i] = a.Value
		}
	}
	return map[string]interface{}{attr.Name: args}
}

func buildSettings(jf *Justfile) map[string]interface{} {
	s := make(map[string]interface{}, len(knownSettingsList))
	for _, sd := range knownSettingsList {
		key := strings.ReplaceAll(sd.Name, "-", "_")
		s[key] = sd.Default
	}

	for _, setting := range collectSettings(jf) {
		key := strings.ReplaceAll(setting.Name, "-", "_")
		switch setting.Value.Kind() {
		case "bool":
			s[key] = *setting.Value.Bool
		case "string":
			s[key] = *setting.Value.String
		case "list":
			if key == "shell" || key == "windows_shell" {
				if len(setting.Value.List) > 0 {
					args := make([]interface{}, 0, len(setting.Value.List)-1)
					for _, a := range setting.Value.List[1:] {
						args = append(args, a)
					}
					s[key] = map[string]interface{}{
						"command":   setting.Value.List[0],
						"arguments": args,
					}
				}
			} else {
				list := make([]interface{}, len(setting.Value.List))
				for i, v := range setting.Value.List {
					list[i] = v
				}
				s[key] = list
			}
		}
	}

	return s
}

func collectSettings(jf *Justfile) []*Setting {
	var settings []*Setting
	for _, imp := range jf.Imports {
		if imp.Justfile != nil {
			settings = append(settings, collectSettings(imp.Justfile)...)
		}
	}
	settings = append(settings, jf.Settings...)
	return settings
}

func toJustDumpJSON(jf *Justfile) ([]byte, error) {
	return json.MarshalIndent(toJustDump(jf, ""), "", "  ")
}

func compareWithJustDump(ours interface{}, theirs interface{}, path string) []string {
	return deepCompare(ours, theirs, path)
}

func deepCompare(a, b interface{}, path string) []string {
	if a == nil && b == nil {
		return nil
	}

	if aNum, ok := toFloat64(a); ok {
		if bNum, ok := toFloat64(b); ok {
			if aNum == bNum {
				return nil
			}
			return []string{path + ": value mismatch: " + jsonStr(a) + " vs " + jsonStr(b)}
		}
	}

	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		return compareMaps(aMap, bMap, path)
	}

	aSlice, aIsSlice := a.([]interface{})
	bSlice, bIsSlice := b.([]interface{})
	if aIsSlice && bIsSlice {
		return compareSlices(aSlice, bSlice, path)
	}

	aJSON := jsonStr(a)
	bJSON := jsonStr(b)
	if aJSON != bJSON {
		return []string{path + ": " + aJSON + " != " + bJSON}
	}
	return nil
}

func compareMaps(a, b map[string]interface{}, path string) []string {
	var diffs []string
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, k := range sorted {
		aVal, aOk := a[k]
		bVal, bOk := b[k]
		p := path + "." + k
		if !aOk {
			diffs = append(diffs, p+": missing in ours (just has "+jsonStr(bVal)+")")
		} else if !bOk {
			diffs = append(diffs, p+": extra in ours ("+jsonStr(aVal)+")")
		} else {
			diffs = append(diffs, deepCompare(aVal, bVal, p)...)
		}
	}
	return diffs
}

func compareSlices(a, b []interface{}, path string) []string {
	var diffs []string
	max := len(a)
	if len(b) > max {
		max = len(b)
	}
	if len(a) != len(b) {
		diffs = append(diffs, path+": length mismatch: "+jsonStr(len(a))+" vs "+jsonStr(len(b)))
	}
	for i := 0; i < max; i++ {
		p := path + "[" + jsonStr(i) + "]"
		if i >= len(a) {
			diffs = append(diffs, p+": missing in ours")
		} else if i >= len(b) {
			diffs = append(diffs, p+": extra in ours")
		} else {
			diffs = append(diffs, deepCompare(a[i], b[i], p)...)
		}
	}
	return diffs
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func jsonStr(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
