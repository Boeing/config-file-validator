package schemastore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var supportedExts = map[string]struct{}{
	"json": {},
	"yaml": {},
	"yml":  {},
	"toml": {},
	"toon": {},
}

type catalogEntry struct {
	FileMatch []string `json:"fileMatch"`
	URL       string   `json:"url"`
}

type catalog struct {
	Schemas []catalogEntry `json:"schemas"`
}

// Store holds a parsed SchemaStore catalog and resolves file paths to local schema files.
type Store struct {
	entries   []catalogEntry
	basePath  string
	schemaDir string
}

// Open reads the SchemaStore catalog from bundlePath and returns a Store.
// The bundlePath should be the root of a SchemaStore clone or download
// containing src/api/json/catalog.json and src/schemas/json/*.json.
func Open(bundlePath string) (*Store, error) {
	catalogPath := filepath.Join(bundlePath, "src", "api", "json", "catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("reading schemastore catalog: %w", err)
	}

	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("parsing schemastore catalog: %w", err)
	}

	filtered := make([]catalogEntry, 0, len(cat.Schemas))
	for _, entry := range cat.Schemas {
		if hasSupported(entry.FileMatch) {
			filtered = append(filtered, entry)
		}
	}

	return &Store{
		entries:   filtered,
		basePath:  bundlePath,
		schemaDir: filepath.Join(bundlePath, "src", "schemas", "json"),
	}, nil
}

// Lookup matches a file path against the catalog and returns the absolute path
// to the local schema file. Only schemas hosted on schemastore.org are resolved.
func (s *Store) Lookup(filePath string) (string, bool) {
	name := filepath.Base(filePath)
	for _, entry := range s.entries {
		for _, pattern := range entry.FileMatch {
			matched, err := matchPattern(pattern, filePath, name)
			if err != nil || !matched {
				continue
			}
			localPath := s.resolveLocal(entry.URL)
			if localPath == "" {
				continue
			}
			if _, err := os.Stat(localPath); err != nil {
				continue
			}
			return localPath, true
		}
	}
	return "", false
}

func (s *Store) resolveLocal(schemaURL string) string {
	const host = "www.schemastore.org"
	if !strings.Contains(schemaURL, host) {
		return ""
	}
	// URL format: https://www.schemastore.org/foo.json -> src/schemas/json/foo.json
	idx := strings.LastIndex(schemaURL, "/")
	if idx < 0 {
		return ""
	}
	filename := schemaURL[idx+1:]
	return filepath.Join(s.schemaDir, filename)
}

func matchPattern(pattern, filePath, baseName string) (bool, error) {
	// Patterns like "package.json" (no glob) match against the base name
	if !strings.ContainsAny(pattern, "*?[") {
		if pattern == baseName {
			return true, nil
		}
		return false, nil
	}
	// Glob patterns match against the full path
	return doublestar.PathMatch(pattern, filePath)
}

func hasSupported(fileMatch []string) bool {
	for _, fm := range fileMatch {
		ext := ""
		if idx := strings.LastIndex(fm, "."); idx >= 0 {
			ext = fm[idx+1:]
		}
		if _, ok := supportedExts[ext]; ok {
			return true
		}
	}
	return false
}
