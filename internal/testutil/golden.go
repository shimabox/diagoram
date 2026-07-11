// Package testutil provides shared test helpers used across
// diagoram's test suites, in particular golden-file comparison.
package testutil

import (
	"flag"
	"os"
	"path/filepath"
)

// update, when set via `go test ./... -update`, makes Golden write
// actual output as the new golden file content instead of comparing
// against the existing one.
var update = flag.Bool("update", false, "update golden files instead of comparing against them")

// TestingT is the subset of *testing.T (or *testing.B) that Golden
// needs. *testing.T satisfies it automatically.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Golden compares actual against the contents of the golden file at
// goldenPath and fails t via Fatalf if they differ.
//
// Run tests with -update (e.g. `go test ./... -update`) to write
// actual as the new golden file content instead of comparing; missing
// parent directories are created automatically.
func Golden(t TestingT, goldenPath, actual string) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("testutil.Golden: cannot create directory for %s: %v", goldenPath, err)
			return
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("testutil.Golden: cannot write golden file %s: %v", goldenPath, err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("testutil.Golden: cannot read golden file %s: %v\nRun `go test ./... -update` to create it.", goldenPath, err)
		return
	}

	if actual != string(want) {
		t.Fatalf("testutil.Golden: %s mismatch\n--- want ---\n%s\n--- got ---\n%s\nRun `go test ./... -update` to update the golden file.", goldenPath, string(want), actual)
	}
}
