package gocode

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpressionRequiresIgnore(t *testing.T) {
	tests := map[string]bool{
		"//go:build ignore":          true,
		"//go:build ignore && tool":  true,
		"//go:build ignore || linux": false,
		"//go:build !ignore":         false,
	}
	for line, want := range tests {
		expr, err := constraint.Parse(line)
		if err != nil {
			t.Fatal(err)
		}
		if got := expressionRequiresIgnore(expr); got != want {
			t.Errorf("expressionRequiresIgnore(%q) = %v, want %v", line, got, want)
		}
	}
}

// writeFiles creates each key in files (a path relative to dir) with
// its value as content, creating parent directories as needed.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", full, err)
		}
	}
}

func TestDiscoverDirsDefaultIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.go":      "package root\n",
		"a_test.go": "package root\n",
		"README.md": "not go\n",
		"pkg/b.go":  "package pkg\n",
	})

	got, err := discoverDirs(dir, ParseOptions{})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}

	want := []dirFiles{
		{Dir: ".", Files: []string{"a.go"}},
		{Dir: "pkg", Files: []string{"b.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsSkipsVendorTestdataAndHidden(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.go":              "package root\n",
		"vendor/v.go":       "package vendor\n",
		"testdata/t.go":     "package testdata\n",
		".hidden/h.go":      "package hidden\n",
		"keep/vendorish.go": "package keep\n", // not literally named "vendor", must be kept
		"keep/testdatum.go": "package keep\n", // not literally named "testdata", must be kept
	})

	got, err := discoverDirs(dir, ParseOptions{})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}

	want := []dirFiles{
		{Dir: ".", Files: []string{"a.go"}},
		{Dir: "keep", Files: []string{"testdatum.go", "vendorish.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsCustomIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.go":      "package root\n",
		"a_gen.go":  "package root\n",
		"a_test.go": "package root\n",
	})

	got, err := discoverDirs(dir, ParseOptions{
		Includes: []string{"*.go"},
		Excludes: []string{"*_gen.go"},
	})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}

	// A custom Excludes list is used as-is: *_test.go is only excluded
	// by default, so specifying Excludes without it means test files
	// are included.
	want := []dirFiles{
		{Dir: ".", Files: []string{"a.go", "a_test.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsIncludeNarrowsFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.go":      "package root\n",
		"b_impl.go": "package root\n",
	})

	got, err := discoverDirs(dir, ParseOptions{
		Includes: []string{"*_impl.go"},
	})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}

	want := []dirFiles{
		{Dir: ".", Files: []string{"b_impl.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsSkipsEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"a.go":         "package root\n",
		"empty/README": "no go files here\n",
	})

	got, err := discoverDirs(dir, ParseOptions{})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}

	want := []dirFiles{
		{Dir: ".", Files: []string{"a.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsCustomDirectoryExcludes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"root.go":                   "package root\n",
		"examples/demo/main.go":     "package main\n",
		"pkg/deep/examples/main.go": "package main\n",
		"pkg/keep.go":               "package pkg\n",
		"pkg/generated/code.go":     "package generated\n",
		"other/generated/code.go":   "package generated\n",
	})

	got, err := discoverDirs(dir, ParseOptions{ExcludeDirs: []string{"examples", "*/generated"}})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}
	want := []dirFiles{
		{Dir: ".", Files: []string{"root.go"}},
		{Dir: "pkg", Files: []string{"keep.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsCustomDirectoryIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"root.go":          "package root\n",
		"pkg/keep.go":      "package pkg\n",
		"pkg/deep/keep.go": "package deep\n",
		"pkg/skip/skip.go": "package skip\n",
		"other/other.go":   "package other\n",
	})

	got, err := discoverDirs(dir, ParseOptions{
		IncludeDirs: []string{"pkg"},
		ExcludeDirs: []string{"skip"},
	})
	if err != nil {
		t.Fatalf("discoverDirs: %v", err)
	}
	want := []dirFiles{
		{Dir: "pkg", Files: []string{"keep.go"}},
		{Dir: "pkg/deep", Files: []string{"keep.go"}},
	}
	assertDirFiles(t, got, want)
}

func TestDiscoverDirsBuildContext(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"base.go":           "package target\n",
		"ignored.go":        "//go:build ignore\n\npackage target\n",
		"target_linux.go":   "package target\n",
		"target_windows.go": "package target\n",
		"tagged.go":         "//go:build custom\n\npackage target\n",
		"other_tagged.go":   "//go:build other\n\npackage target\n",
	})

	union, err := discoverDirs(dir, ParseOptions{})
	if err != nil {
		t.Fatalf("discoverDirs union: %v", err)
	}
	if len(union) != 1 || len(union[0].Files) != 5 {
		t.Fatalf("union files = %+v, want all variants except build ignore", union)
	}

	selected, err := discoverDirs(dir, ParseOptions{BuildContext: &BuildContext{
		GOOS: "linux", GOARCH: "amd64", Tags: []string{"custom"},
	}})
	if err != nil {
		t.Fatalf("discoverDirs selected: %v", err)
	}
	want := []dirFiles{{Dir: ".", Files: []string{"base.go", "tagged.go", "target_linux.go"}}}
	assertDirFiles(t, selected, want)

	withIgnore, err := discoverDirs(dir, ParseOptions{BuildContext: &BuildContext{
		GOOS: "linux", GOARCH: "amd64", Tags: []string{"ignore"},
	}})
	if err != nil {
		t.Fatalf("discoverDirs ignore tag: %v", err)
	}
	if len(withIgnore) != 1 || !containsString(withIgnore[0].Files, "ignored.go") {
		t.Fatalf("files with explicit ignore tag = %+v, want ignored.go", withIgnore)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertDirFiles(t *testing.T, got []dirFiles, want []dirFiles) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("discoverDirs returned %d dirs, want %d\ngot:  %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Dir != want[i].Dir {
			t.Errorf("dir[%d].Dir = %q, want %q", i, got[i].Dir, want[i].Dir)
		}
		if !reflect.DeepEqual(got[i].Files, want[i].Files) {
			t.Errorf("dir[%d].Files = %v, want %v", i, got[i].Files, want[i].Files)
		}
	}
}
