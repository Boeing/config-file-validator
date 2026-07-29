package formatter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// FormatIgnores holds per-format ignore patterns loaded from external tool
// configs. Created during config resolution in Tier 2 (no .cfv.toml).
// Nil in Tier 1 or when --no-config is set.
//
// Ignore files only affect format checking — not syntax or schema validation.
// A file in .prettierignore still gets its syntax validated; it just doesn't
// get a ~ for formatting.
type FormatIgnores struct {
	// prettierPatterns applies to json, jsonc, yaml (when prettier owns yaml).
	prettierPatterns []gitignore.Pattern
	prettierBaseDir  string

	// taploPatterns applies to toml only.
	taploPatterns []gitignore.Pattern
	taploBaseDir  string

	// yamlfmtPatterns applies to yaml (when yamlfmt owns yaml).
	yamlfmtPatterns []gitignore.Pattern
	yamlfmtBaseDir  string

	// yamlfmtOwnsYAML is true when .yamlfmt was found (takes precedence over prettier for YAML).
	yamlfmtOwnsYAML bool
}

// ShouldSkipFormat returns true if the file at the given absolute path should
// be skipped from format checking/formatting for the given format name.
func (fi *FormatIgnores) ShouldSkipFormat(absPath string, formatName string) bool {
	if fi == nil {
		return false
	}

	switch formatName {
	case "json", "jsonc":
		return fi.matchPrettier(absPath)
	case "yaml":
		if fi.yamlfmtOwnsYAML {
			return fi.matchYamlfmt(absPath)
		}
		return fi.matchPrettier(absPath)
	case "toml":
		return fi.matchTaplo(absPath)
	default:
		return false
	}
}

func (fi *FormatIgnores) matchPrettier(absPath string) bool {
	if len(fi.prettierPatterns) == 0 {
		return false
	}
	return matchPatterns(fi.prettierPatterns, fi.prettierBaseDir, absPath)
}

func (fi *FormatIgnores) matchTaplo(absPath string) bool {
	if len(fi.taploPatterns) == 0 {
		return false
	}
	return matchPatterns(fi.taploPatterns, fi.taploBaseDir, absPath)
}

func (fi *FormatIgnores) matchYamlfmt(absPath string) bool {
	if len(fi.yamlfmtPatterns) == 0 {
		return false
	}
	return matchPatterns(fi.yamlfmtPatterns, fi.yamlfmtBaseDir, absPath)
}

// matchPatterns checks if absPath matches any of the patterns, resolved
// relative to baseDir.
func matchPatterns(patterns []gitignore.Pattern, baseDir, absPath string) bool {
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false // file is outside the config's scope
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	matcher := gitignore.NewMatcher(patterns)
	return matcher.Match(parts, false)
}

// BuildFormatIgnores constructs a FormatIgnores from the loaded tool configs.
// Returns nil if no ignore patterns are found.
//
// prettierIgnoreDir is the directory to search for .prettierignore (walk-up from search root).
// taploCfg and yamlfmtCfg are the loaded tool configs (may be nil).
func BuildFormatIgnores(prettierIgnoreDir string, taploCfg *Taplo, yamlfmtCfg *Yamlfmt) *FormatIgnores {
	fi := &FormatIgnores{}

	// Prettier: look for .prettierignore by walking up from the search root.
	if prettierIgnoreDir != "" {
		patterns, baseDir := FindPrettierIgnore(prettierIgnoreDir)
		if len(patterns) > 0 {
			fi.prettierPatterns = patterns
			fi.prettierBaseDir = baseDir
		}
	}

	// Taplo: read exclude array from taplo.toml.
	if taploCfg != nil && len(taploCfg.Exclude) > 0 {
		var patterns []gitignore.Pattern
		for _, glob := range taploCfg.Exclude {
			patterns = append(patterns, gitignore.ParsePattern(glob, nil))
		}
		fi.taploPatterns = patterns
		fi.taploBaseDir = taploCfg.ConfigDir
	}

	// Yamlfmt: read exclude array from .yamlfmt + load .yamlfmtignore.
	if yamlfmtCfg != nil {
		fi.yamlfmtOwnsYAML = true

		var patterns []gitignore.Pattern
		// Config exclude key.
		for _, glob := range yamlfmtCfg.Exclude {
			patterns = append(patterns, gitignore.ParsePattern(glob, nil))
		}
		// .yamlfmtignore file (same directory as .yamlfmt).
		ignoreFile := filepath.Join(yamlfmtCfg.ConfigDir, ".yamlfmtignore")
		if filePatterns := loadIgnoreFile(ignoreFile); len(filePatterns) > 0 {
			patterns = append(patterns, filePatterns...)
		}

		if len(patterns) > 0 {
			fi.yamlfmtPatterns = patterns
			fi.yamlfmtBaseDir = yamlfmtCfg.ConfigDir
		}
	}

	// Return nil if no patterns loaded (avoid per-file matching overhead).
	if len(fi.prettierPatterns) == 0 && len(fi.taploPatterns) == 0 && len(fi.yamlfmtPatterns) == 0 {
		return nil
	}
	return fi
}

// FindPrettierIgnore walks up from startDir looking for a .prettierignore file.
// Returns the parsed patterns and the directory where the file was found.
// Returns nil, "" if no .prettierignore is found.
func FindPrettierIgnore(startDir string) ([]gitignore.Pattern, string) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, ""
	}
	for {
		path := filepath.Join(dir, ".prettierignore")
		patterns := loadIgnoreFile(path)
		if patterns != nil {
			return patterns, dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, ""
}

// loadIgnoreFile reads a gitignore-style file and returns parsed patterns.
// Returns nil if the file does not exist or is empty (no patterns).
func loadIgnoreFile(path string) []gitignore.Pattern {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return ParseGitignorePatterns(data)
}

// ParseGitignorePatterns parses gitignore-style content (one pattern per line,
// # for comments, ! for negation, blank lines skipped) into patterns.
func ParseGitignorePatterns(data []byte) []gitignore.Pattern {
	var patterns []gitignore.Pattern
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}
	if len(patterns) == 0 {
		return nil
	}
	return patterns
}
