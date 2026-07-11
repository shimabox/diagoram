package gocode

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dirFiles is one directory's set of analyzable Go files, discovered
// by discoverDirs.
type dirFiles struct {
	// Dir is the directory's path relative to rootDir ("." for the
	// root itself).
	Dir string
	// AbsDir is the directory's absolute (or rootDir-joined) path, for
	// opening files.
	AbsDir string
	// Files are the matching file base names, sorted.
	Files []string
}

// defaultIncludes and defaultExcludes are the glob sets ParseOptions
// falls back to when the corresponding field is empty.
var (
	defaultIncludes = []string{"*.go"}
	defaultExcludes = []string{"*_test.go"}
)

// skipDirNames are directory base names that are never descended into
// or treated as packages.
var skipDirNames = map[string]bool{
	"vendor":   true,
	"testdata": true,
}

// discoverDirs walks rootDir and returns, in deterministic path order,
// every directory that contains at least one file matching opt's
// include/exclude globs. Directories named "vendor" or "testdata", and
// any directory whose base name starts with ".", are skipped entirely
// (not walked into, not returned).
func discoverDirs(rootDir string, opt ParseOptions) ([]dirFiles, error) {
	includes := opt.Includes
	if len(includes) == 0 {
		includes = defaultIncludes
	}
	excludes := opt.Excludes
	if len(excludes) == 0 {
		excludes = defaultExcludes
	}

	var results []dirFiles
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != rootDir {
			base := d.Name()
			if skipDirNames[base] || strings.HasPrefix(base, ".") {
				return fs.SkipDir
			}
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			if !matchAny(includes, name) {
				continue
			}
			if matchAny(excludes, name) {
				continue
			}
			files = append(files, name)
		}
		if len(files) == 0 {
			return nil
		}
		sort.Strings(files)

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		results = append(results, dirFiles{Dir: rel, AbsDir: path, Files: files})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Dir < results[j].Dir })
	return results, nil
}

// matchAny reports whether name matches at least one of the glob
// patterns. Malformed patterns (filepath.ErrBadPattern) never match.
func matchAny(patterns []string, name string) bool {
	for _, p := range patterns {
		if ok, err := filepath.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}
