package finder

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	extCache         map[string]struct{}
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
		normalized := append([]TypeOverride(nil), overrides...)
		for i := range normalized {
			normalized[i].Pattern = filepath.ToSlash(normalized[i].Pattern)
		}
		fsf.TypeOverrides = normalized
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
	finder := fsf
	finder.extCache = make(map[string]struct{})

	seen := make(map[string]struct{}, 0)
	uniqueMatches := make([]FileMetadata, 0)
	for _, pathRoot := range finder.PathRoots {
		// remove all leading and trailing whitespace
		trimmedPathRoot := strings.TrimSpace(pathRoot)
		matches, err := finder.findOne(trimmedPathRoot, seen)
		if err != nil {
			return nil, err
		}
		uniqueMatches = append(uniqueMatches, matches...)
	}
	return uniqueMatches, nil
}

// MatchFile applies the finder filters to a single file path.
func (fsf FileSystemFinder) MatchFile(path string) ([]FileMetadata, error) {
	finder := fsf
	finder.extCache = make(map[string]struct{})

	path = strings.TrimSpace(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, 1)
	matches := make([]FileMetadata, 0, 1)
	for _, pathRoot := range finder.PathRoots {
		if err := finder.matchFileInRoot(absPath, info, strings.TrimSpace(pathRoot), seen, &matches); err != nil {
			return nil, err
		}
	}
	return matches, nil
}

func (fsf *FileSystemFinder) matchFileInRoot(
	absPath string,
	info os.FileInfo,
	pathRoot string,
	seen map[string]struct{},
	matches *[]FileMetadata,
) error {
	if pathRoot == "" {
		return nil
	}

	pathRoot = strings.TrimRight(pathRoot, string(os.PathSeparator))
	rootAbs, err := filepath.Abs(pathRoot)
	if err != nil {
		return err
	}
	rootInfo, err := os.Stat(rootAbs)
	if err != nil {
		return err
	}

	if !rootInfo.IsDir() {
		if sameFilesystemPath(rootAbs, absPath) {
			return fsf.handleFile(absPath, fs.FileInfoToDirEntry(info), seen, matches)
		}
		return nil
	}
	if !containsPath(rootAbs, absPath) {
		return nil
	}

	if err := fsf.checkDirsForFile(rootAbs, filepath.Dir(absPath)); err != nil {
		if errors.Is(err, filepath.SkipDir) {
			return nil
		}
		return err
	}

	if fsf.Gitignore {
		gim := newGitignoreMatcher(rootAbs)
		if gim != nil && gitignoreMatchesPath(gim, rootAbs, absPath) {
			return nil
		}
	}

	return fsf.handleFile(absPath, fs.FileInfoToDirEntry(info), seen, matches)
}

func (fsf *FileSystemFinder) checkDirsForFile(rootAbs, fileDir string) error {
	var depth int
	if fsf.Depth != nil {
		depth = *fsf.Depth
	}
	maxDepth := strings.Count(rootAbs, string(os.PathSeparator)) + depth

	for _, dir := range dirsFromRoot(rootAbs, fileDir) {
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if err := fsf.handleDir(dir, fs.FileInfoToDirEntry(info), maxDepth); err != nil {
			return err
		}
	}
	return nil
}

func gitignoreMatchesPath(gim *gitignoreMatcher, rootAbs, absPath string) bool {
	for _, dir := range dirsFromRoot(rootAbs, filepath.Dir(absPath)) {
		if dir != rootAbs && gim.match(dir, true) {
			return true
		}
		gim.pushDir(dir)
	}
	return gim.match(absPath, false)
}

func dirsFromRoot(rootAbs, fileDir string) []string {
	dirs := []string{rootAbs}
	relDir, err := filepath.Rel(rootAbs, fileDir)
	if err != nil || relDir == "." {
		return dirs
	}

	dir := rootAbs
	for _, part := range strings.Split(relDir, string(os.PathSeparator)) {
		dir = filepath.Join(dir, part)
		dirs = append(dirs, dir)
	}
	return dirs
}

func containsPath(rootAbs, childAbs string) bool {
	rel, err := filepath.Rel(rootAbs, childAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func sameFilesystemPath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// findOne recursively walks through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array for a
// single path and returns the file metadata.
func (fsf *FileSystemFinder) findOne(pathRoot string, seenMap map[string]struct{}) ([]FileMetadata, error) {
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

func (fsf *FileSystemFinder) handleDir(path string, dirEntry fs.DirEntry, maxDepth int) error {
	_, isExcluded := fsf.ExcludeDirs[dirEntry.Name()]
	if isExcluded || (fsf.Depth != nil && strings.Count(path, string(os.PathSeparator)) > maxDepth) {
		return filepath.SkipDir
	}
	return nil
}

func (fsf *FileSystemFinder) handleFile(path string, dirEntry fs.DirEntry, seenMap map[string]struct{}, matchingFiles *[]FileMetadata) error {
	walkFileName := filepath.Base(path)
	walkFileExtension := strings.TrimPrefix(filepath.Ext(path), ".")
	extensionLowerCase := strings.ToLower(walkFileExtension)
	pathForPatternMatch := filepath.ToSlash(path)

	// Check type overrides first (user-specified mappings take priority)
	for _, override := range fsf.TypeOverrides {
		matched, err := doublestar.PathMatch(override.Pattern, pathForPatternMatch)
		if err != nil {
			return err
		}
		if matched {
			return fsf.addFileIfNotExcluded(path, dirEntry, override.FileType, seenMap, matchingFiles)
		}
	}

	// Fall back to built-in file types.
	// KnownFiles matches take priority over extension matches so that
	// files like tsconfig.json resolve to jsonc (not json).
	for _, fileType := range fsf.FileTypes {
		if _, isKnownFile := fileType.KnownFiles[walkFileName]; isKnownFile {
			return fsf.addFileIfNotExcluded(path, dirEntry, fileType, seenMap, matchingFiles)
		}
	}
	if fsf.isExtensionCached(extensionLowerCase) {
		return nil
	}
	for _, fileType := range fsf.FileTypes {
		if _, hasExtension := fileType.Extensions[extensionLowerCase]; hasExtension {
			return fsf.addFileIfNotExcluded(path, dirEntry, fileType, seenMap, matchingFiles)
		}
	}

	fsf.cacheUnsupportedExtension(extensionLowerCase)
	return nil
}

func (fsf *FileSystemFinder) isExtensionCached(extension string) bool {
	if len(fsf.TypeOverrides) > 0 || extension == "" || fsf.extCache == nil {
		return false
	}
	_, isCached := fsf.extCache[extension]
	return isCached
}

func (fsf *FileSystemFinder) cacheUnsupportedExtension(extension string) {
	if len(fsf.TypeOverrides) > 0 || extension == "" || fsf.extCache == nil {
		return
	}
	fsf.extCache[extension] = struct{}{}
}

func (fsf *FileSystemFinder) addFileIfNotExcluded(path string, dirEntry fs.DirEntry, fileType filetype.FileType, seenMap map[string]struct{}, matchingFiles *[]FileMetadata) error {
	if fsf.isFileTypeExcluded(fileType) {
		return nil
	}
	return fsf.addFile(path, dirEntry, fileType, seenMap, matchingFiles)
}

func (fsf *FileSystemFinder) isFileTypeExcluded(fileType filetype.FileType) bool {
	if _, isExcluded := fsf.ExcludeFileTypes[fileType.Name]; isExcluded {
		return true
	}
	for extension := range fileType.Extensions {
		if _, isExcluded := fsf.ExcludeFileTypes[extension]; isExcluded {
			return true
		}
	}
	return false
}

func (*FileSystemFinder) addFile(path string, dirEntry fs.DirEntry, fileType filetype.FileType, seenMap map[string]struct{}, matchingFiles *[]FileMetadata) error {
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
