package finder

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/tools"
)

type TypeOverride struct {
	Pattern  string
	FileType filetype.FileType
}

type FileSystemFinder struct {
	PathRoots        []string
	FileTypes        []filetype.FileType
	ExcludeDirs      map[string]struct{}
	ExcludeFileTypes map[string]struct{}
	Depth            *int
	TypeOverrides    []TypeOverride
	Gitignore        bool
}

type FSFinderOptions func(*FileSystemFinder)

// Set the CLI SearchPath
func WithPathRoots(paths ...string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.PathRoots = paths
	}
}

// Add a custom list of file types to the FSFinder
func WithFileTypes(fileTypes []filetype.FileType) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.FileTypes = fileTypes
	}
}

// Add a custom list of file types to the FSFinder
func WithExcludeDirs(excludeDirs []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeDirs = tools.ArrToMap(excludeDirs...)
	}
}

// WithExcludeFileTypes adds excluded file types to FSFinder.
func WithExcludeFileTypes(types []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeFileTypes = tools.ArrToMap(types...)
	}
}

// WithDepth adds the depth for search recursion to FSFinder
func WithDepth(depthVal int) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.Depth = &depthVal
	}
}

// WithTypeOverrides adds glob pattern to file type mappings
func WithTypeOverrides(overrides []TypeOverride) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.TypeOverrides = overrides
	}
}

// WithGitignore enables skipping files/directories matched by .gitignore patterns
func WithGitignore(enabled bool) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.Gitignore = enabled
	}
}

func FileSystemFinderInit(opts ...FSFinderOptions) *FileSystemFinder {
	defaultExcludeDirs := make(map[string]struct{})
	defaultExcludeFileTypes := make(map[string]struct{})
	defaultPathRoots := []string{"."}

	fsfinder := &FileSystemFinder{
		PathRoots:        defaultPathRoots,
		FileTypes:        filetype.FileTypes,
		ExcludeDirs:      defaultExcludeDirs,
		ExcludeFileTypes: defaultExcludeFileTypes,
	}

	for _, opt := range opts {
		opt(fsfinder)
	}

	return fsfinder
}

// Find implements the FileFinder interface by calling findOne on
// all the PathRoots and providing the aggregated FileMetadata after
// ignoring all the duplicate files
func (fsf FileSystemFinder) Find() ([]FileMetadata, error) {
	seen := make(map[string]struct{}, 0)
	uniqueMatches := make([]FileMetadata, 0)
	for _, pathRoot := range fsf.PathRoots {
		// remove all leading and trailing whitespace
		trimmedPathRoot := strings.TrimSpace(pathRoot)
		matches, err := fsf.findOne(trimmedPathRoot, seen)
		if err != nil {
			return nil, err
		}
		uniqueMatches = append(uniqueMatches, matches...)
	}
	return uniqueMatches, nil
}

// findOne recursively walks through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array for a
// single path and returns the file metadata.
func (fsf FileSystemFinder) findOne(pathRoot string, seenMap map[string]struct{}) ([]FileMetadata, error) {
	var matchingFiles []FileMetadata

	pathRoot = strings.TrimRight(pathRoot, string(os.PathSeparator))

	if _, err := os.Stat(pathRoot); os.IsNotExist(err) {
		return nil, err
	}

	var depth int
	if fsf.Depth != nil {
		depth = *fsf.Depth
	}

	var gim *gitignoreMatcher
	if fsf.Gitignore {
		gim = newGitignoreMatcher(pathRoot)
		if gim != nil {
			// Use absolute pathRoot so WalkDir paths match gim.absPathRoot.
			pathRoot = gim.absPathRoot
		}
	}

	maxDepth := strings.Count(pathRoot, string(os.PathSeparator)) + depth

	err := filepath.WalkDir(pathRoot,
		func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if dirEntry.IsDir() {
				if gim != nil {
					if path != pathRoot && gim.match(path, true) {
						return filepath.SkipDir
					}
					gim.pushDir(path)
				}
				return fsf.handleDir(path, dirEntry, maxDepth)
			}

			if gim != nil && gim.match(path, false) {
				return nil
			}

			return fsf.handleFile(path, dirEntry, seenMap, &matchingFiles)
		})
	if err != nil {
		return nil, err
	}

	return matchingFiles, nil
}

// gitignoreMatcher lazily loads .gitignore files during WalkDir.
type gitignoreMatcher struct {
	patterns    []gitignore.Pattern
	matcher     gitignore.Matcher
	absPathRoot string
}

func newGitignoreMatcher(pathRoot string) *gitignoreMatcher {
	absPathRoot, err := filepath.Abs(pathRoot)
	if err != nil {
		return nil
	}
	repoRoot := findRepoRoot(absPathRoot)
	if repoRoot == "" {
		return nil
	}

	gim := &gitignoreMatcher{absPathRoot: absPathRoot}

	// Load system and global patterns (reads 1-2 config files).
	// Errors are ignored — a missing or broken gitconfig should not
	// prevent the matcher from working with local .gitignore files.
	rootFS := osfs.New("/")
	ps, _ := gitignore.LoadSystemPatterns(rootFS)
	gim.patterns = append(gim.patterns, ps...)
	ps, _ = gitignore.LoadGlobalPatterns(rootFS)
	gim.patterns = append(gim.patterns, ps...)

	// Load .git/info/exclude from the repo root.
	gim.loadFile(filepath.Join(repoRoot, ".git", "info", "exclude"), nil)

	// Load .gitignore files from repo root up to (not including) the search path.
	// The search path's own .gitignore is loaded by pushDir during WalkDir.
	// Patterns from ancestor directories use nil domain so they apply globally.
	if repoRoot != absPathRoot {
		rel, _ := filepath.Rel(repoRoot, absPathRoot)
		segments := strings.Split(rel, string(os.PathSeparator))
		cur := repoRoot
		for _, seg := range segments {
			gim.loadFile(filepath.Join(cur, ".gitignore"), nil)
			cur = filepath.Join(cur, seg)
		}
	}

	gim.matcher = gitignore.NewMatcher(gim.patterns)
	return gim
}

// pushDir loads a .gitignore from the given directory if one exists.
func (gim *gitignoreMatcher) pushDir(dir string) {
	before := len(gim.patterns)
	rel, _ := filepath.Rel(gim.absPathRoot, dir)
	var domain []string
	if rel != "" && rel != "." {
		domain = strings.Split(rel, string(os.PathSeparator))
	}
	gim.loadFile(filepath.Join(dir, ".gitignore"), domain)
	if len(gim.patterns) != before {
		gim.matcher = gitignore.NewMatcher(gim.patterns)
	}
}

func (gim *gitignoreMatcher) loadFile(path string, domain []string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			line = strings.TrimPrefix(line, "\xef\xbb\xbf")
			first = false
		}
		if len(strings.TrimSpace(line)) > 0 && !strings.HasPrefix(line, "#") {
			gim.patterns = append(gim.patterns, gitignore.ParsePattern(line, domain))
		}
	}
}

func (gim *gitignoreMatcher) match(path string, isDir bool) bool {
	relPath, _ := filepath.Rel(gim.absPathRoot, path)
	parts := strings.Split(relPath, string(os.PathSeparator))
	return gim.matcher.Match(parts, isDir)
}

// findRepoRoot walks up from an absolute path looking for a .git directory.
func findRepoRoot(absDir string) string {
	for dir := absDir; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func (fsf FileSystemFinder) handleDir(path string, dirEntry fs.DirEntry, maxDepth int) error {
	_, isExcluded := fsf.ExcludeDirs[dirEntry.Name()]
	if isExcluded || (fsf.Depth != nil && strings.Count(path, string(os.PathSeparator)) > maxDepth) {
		return filepath.SkipDir
	}
	return nil
}

func (fsf FileSystemFinder) handleFile(path string, dirEntry fs.DirEntry, seenMap map[string]struct{}, matchingFiles *[]FileMetadata) error {
	walkFileName := filepath.Base(path)
	walkFileExtension := strings.TrimPrefix(filepath.Ext(path), ".")
	extensionLowerCase := strings.ToLower(walkFileExtension)

	if _, isExcluded := fsf.ExcludeFileTypes[extensionLowerCase]; isExcluded {
		return nil
	}

	// Check type overrides first (user-specified mappings take priority)
	for _, override := range fsf.TypeOverrides {
		matched, err := doublestar.PathMatch(override.Pattern, path)
		if err != nil {
			return err
		}
		if matched {
			return fsf.addFile(path, dirEntry, override.FileType, seenMap, matchingFiles)
		}
	}

	// Fall back to built-in file types.
	// KnownFiles matches take priority over extension matches so that
	// files like tsconfig.json resolve to jsonc (not json).
	for _, fileType := range fsf.FileTypes {
		if _, isKnownFile := fileType.KnownFiles[walkFileName]; isKnownFile {
			return fsf.addFile(path, dirEntry, fileType, seenMap, matchingFiles)
		}
	}
	for _, fileType := range fsf.FileTypes {
		if _, hasExtension := fileType.Extensions[extensionLowerCase]; hasExtension {
			return fsf.addFile(path, dirEntry, fileType, seenMap, matchingFiles)
		}
	}

	// Only cache exclusion if no type overrides are configured.
	// Never cache "" (extensionless files) — one unrecognized extensionless
	// file (e.g. .gitignore) must not prevent known extensionless files
	// (e.g. Pipfile) from being found later.
	if len(fsf.TypeOverrides) == 0 && extensionLowerCase != "" {
		fsf.ExcludeFileTypes[extensionLowerCase] = struct{}{}
	}
	return nil
}

func (FileSystemFinder) addFile(path string, dirEntry fs.DirEntry, fileType filetype.FileType, seenMap map[string]struct{}, matchingFiles *[]FileMetadata) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if _, seen := seenMap[absPath]; !seen {
		*matchingFiles = append(*matchingFiles, FileMetadata{dirEntry.Name(), absPath, fileType})
		seenMap[absPath] = struct{}{}
	}

	return nil
}
