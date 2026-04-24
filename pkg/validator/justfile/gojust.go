package gojust

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Parse parses justfile content from bytes. Import and module statements
// are represented as AST nodes but not resolved — use ParseFile to
// automatically resolve imports and modules from disk.
func Parse(content []byte) (*Justfile, error) {
	l := newLexer(content)
	tokens, err := l.lex()
	if err != nil {
		return nil, err
	}

	p := newParser(tokens, "")
	return p.parse()
}

// ParseFile reads and parses a justfile from disk. Imports and modules
// are resolved recursively relative to the file's directory.
//
// Security note: ParseFile follows import and module paths specified in
// the justfile, which may reference files outside the justfile's directory
// (including absolute paths and ~/). Do not use ParseFile on untrusted
// justfiles if file read side effects are a concern. Use [Parse] instead
// to parse without filesystem access.
func ParseFile(filename string) (*Justfile, error) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	return parseFileRecursive(absPath, make(map[string]bool), 0)
}

const maxImportDepth = 100

func parseFileRecursive(absPath string, inProgress map[string]bool, depth int) (*Justfile, error) {
	if depth > maxImportDepth {
		return nil, &ParseError{
			Message: "import depth limit exceeded (possible import chain too deep)",
		}
	}
	if inProgress[absPath] {
		return nil, &ParseError{
			Message: "circular import: " + absPath,
		}
	}
	inProgress[absPath] = true
	defer delete(inProgress, absPath)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	l := newLexer(content)
	tokens, err := l.lex()
	if err != nil {
		var pe *ParseError
		if errors.As(err, &pe) {
			pe.File = absPath
		}
		return nil, err
	}

	p := newParser(tokens, absPath)
	jf, err := p.parse()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(absPath)

	// Resolve imports
	for _, imp := range jf.Imports {
		impPath := resolveImportPath(dir, imp.Path)
		resolved, err := parseFileRecursive(impPath, inProgress, depth+1)
		if err != nil {
			if imp.Optional && os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		imp.Justfile = resolved
	}

	// Resolve modules
	for _, mod := range jf.Modules {
		modPath, err := resolveModulePath(dir, mod)
		if err != nil {
			if mod.Optional {
				continue
			}
			return nil, err
		}
		resolved, err := parseFileRecursive(modPath, inProgress, depth+1)
		if err != nil {
			if mod.Optional && os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		mod.Justfile = resolved
	}

	return jf, nil
}

func resolveImportPath(dir, path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

func resolveModulePath(dir string, mod *Module) (string, error) {
	if mod.Path != "" {
		return resolveImportPath(dir, mod.Path), nil
	}

	// Search order: name.just, name/mod.just, name/justfile, name/.justfile
	candidates := []string{
		filepath.Join(dir, mod.Name+".just"),
		filepath.Join(dir, mod.Name, "mod.just"),
		filepath.Join(dir, mod.Name, "justfile"),
		filepath.Join(dir, mod.Name, "Justfile"),
		filepath.Join(dir, mod.Name, ".justfile"),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", &ParseError{
		Pos:     mod.Pos,
		Message: "could not find module file for '" + mod.Name + "'",
	}
}
