package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeT is a minimal TestingT that records failures instead of
// stopping execution, so we can assert on Golden's failure behavior
// in-process (a real *testing.T's Fatalf would abort the test).
type fakeT struct {
	failed  bool
	message string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
	f.message = fmt.Sprintf(format, args...)
}

func TestGoldenMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.golden")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ft := &fakeT{}
	Golden(ft, path, "hello\n")

	if ft.failed {
		t.Errorf("Golden() failed unexpectedly: %s", ft.message)
	}
}

func TestGoldenMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.golden")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ft := &fakeT{}
	Golden(ft, path, "goodbye\n")

	if !ft.failed {
		t.Fatal("Golden() did not fail on mismatch")
	}
	if !strings.Contains(ft.message, path) {
		t.Errorf("Golden() failure message = %q, want it to mention %q", ft.message, path)
	}
}

func TestGoldenMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.golden")

	ft := &fakeT{}
	Golden(ft, path, "content")

	if !ft.failed {
		t.Fatal("Golden() did not fail when the golden file is missing")
	}
	if !strings.Contains(ft.message, "-update") {
		t.Errorf("Golden() failure message = %q, want it to mention -update", ft.message)
	}
}

func TestGoldenUpdateWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "example.golden")

	orig := *update
	*update = true
	defer func() { *update = orig }()

	ft := &fakeT{}
	Golden(ft, path, "new content\n")

	if ft.failed {
		t.Fatalf("Golden() failed unexpectedly during update: %s", ft.message)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected golden file to be created: %v", err)
	}
	if string(got) != "new content\n" {
		t.Errorf("golden file content = %q, want %q", string(got), "new content\n")
	}
}
