package gocode

import (
	"bufio"
	"go/build"
	"go/build/constraint"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
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
	generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)
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
		rel := "."
		if path != rootDir {
			base := d.Name()
			if skipDirNames[base] || strings.HasPrefix(base, ".") {
				return fs.SkipDir
			}
			var err error
			rel, err = filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			if matchAnyPath(opt.ExcludeDirs, filepath.ToSlash(rel)) {
				return fs.SkipDir
			}
		}
		rel = filepath.ToSlash(rel)
		if len(opt.IncludeDirs) > 0 && !matchIncludedPath(opt.IncludeDirs, rel) {
			return nil
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
			if opt.GeneratedFiles != GeneratedFilesAll {
				generated, err := isGeneratedGoFile(filepath.Join(path, name))
				if err != nil {
					return err
				}
				if opt.GeneratedFiles == GeneratedFilesExclude && generated {
					continue
				}
				if opt.GeneratedFiles == GeneratedFilesOnly && !generated {
					continue
				}
			}
			if opt.BuildContext == nil {
				ignored, err := requiresIgnoreBuildTag(filepath.Join(path, name))
				if err != nil {
					return err
				}
				if ignored {
					continue
				}
			}
			matches, err := matchBuildContext(path, name, opt.BuildContext)
			if err != nil {
				return err
			}
			if !matches {
				continue
			}
			files = append(files, name)
		}
		if len(files) == 0 {
			return nil
		}
		sort.Strings(files)

		results = append(results, dirFiles{Dir: rel, AbsDir: path, Files: files})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Dir < results[j].Dir })
	return results, nil
}

func isGeneratedGoFile(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if generatedMarker.MatchString(scanner.Text()) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func readBuildExpression(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var legacy []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			if constraint.IsGoBuild(line) {
				expr, err := constraint.Parse(line)
				if err != nil {
					return "", err
				}
				return expr.String(), nil
			}
			if constraint.IsPlusBuild(line) {
				expr, err := constraint.Parse(line)
				if err != nil {
					return "", err
				}
				legacy = append(legacy, expr.String())
			}
			continue
		}
		break
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(legacy) == 1 {
		return legacy[0], nil
	}
	for i := range legacy {
		legacy[i] = "(" + legacy[i] + ")"
	}
	return strings.Join(legacy, " && "), nil
}

func matchIncludedPath(patterns []string, name string) bool {
	for current := name; ; current = pathpkg.Dir(current) {
		if matchAnyPath(patterns, current) {
			return true
		}
		if current == "." {
			return false
		}
	}
}

func requiresIgnoreBuildTag(filename string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) {
				expr, err := constraint.Parse(line)
				if err != nil {
					return false, err
				}
				if expressionRequiresIgnore(expr) {
					return true, nil
				}
			}
			continue
		}
		break
	}
	return false, scanner.Err()
}

func expressionRequiresIgnore(expr constraint.Expr) bool {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		return e.Tag == "ignore"
	case *constraint.AndExpr:
		return expressionRequiresIgnore(e.X) || expressionRequiresIgnore(e.Y)
	case *constraint.OrExpr:
		return expressionRequiresIgnore(e.X) && expressionRequiresIgnore(e.Y)
	default:
		return false
	}
}

func matchBuildContext(dir, name string, selected *BuildContext) (bool, error) {
	if selected == nil {
		return true, nil
	}
	context := build.Default
	if selected.GOOS != "" {
		context.GOOS = selected.GOOS
	}
	if selected.GOARCH != "" {
		context.GOARCH = selected.GOARCH
	}
	context.BuildTags = append([]string(nil), selected.Tags...)
	return context.MatchFile(dir, name)
}

func matchAnyPath(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if ok, err := pathpkg.Match(pattern, name); err == nil && ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, err := pathpkg.Match(pattern, pathpkg.Base(name)); err == nil && ok {
				return true
			}
		}
	}
	return false
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
