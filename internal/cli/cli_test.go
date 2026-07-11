package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/testutil"
)

// fixturesDir is the shared testdata root, relative to this package.
const fixturesDir = "../../testdata/fixtures"

func TestRun(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantCode      int
		wantStdoutHas string
		wantStderrHas string
	}{
		{
			name:          "no arguments prints usage error",
			args:          []string{},
			wantCode:      1,
			wantStderrHas: "Usage: diagoram",
		},
		{
			name:          "nonexistent directory prints a helpful error",
			args:          []string{"/no/such/dir/does-not-exist-diagoram-test"},
			wantCode:      1,
			wantStderrHas: "does not exist",
		},
		{
			name:          "-h prints usage and exits 0",
			args:          []string{"-h"},
			wantCode:      0,
			wantStdoutHas: "Usage: diagoram",
		},
		{
			name:          "--help prints usage and exits 0",
			args:          []string{"--help"},
			wantCode:      0,
			wantStdoutHas: "Usage: diagoram",
		},
		{
			name:          "-v prints version and exits 0",
			args:          []string{"-v"},
			wantCode:      0,
			wantStdoutHas: "diagoram version",
		},
		{
			name:          "--version prints version and exits 0",
			args:          []string{"--version"},
			wantCode:      0,
			wantStdoutHas: "diagoram version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run(tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("Run(%v) exit code = %d, want %d (stdout=%q stderr=%q)", tt.args, code, tt.wantCode, stdout.String(), stderr.String())
			}
			if tt.wantStdoutHas != "" && !strings.Contains(stdout.String(), tt.wantStdoutHas) {
				t.Errorf("Run(%v) stdout = %q, want substring %q", tt.args, stdout.String(), tt.wantStdoutHas)
			}
			if tt.wantStderrHas != "" && !strings.Contains(stderr.String(), tt.wantStderrHas) {
				t.Errorf("Run(%v) stderr = %q, want substring %q", tt.args, stderr.String(), tt.wantStderrHas)
			}
		})
	}
}

func TestRunValidDirectory(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{dir}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("Run([%q]) exit code = %d, want 0 (stdout=%q stderr=%q)", dir, code, stdout.String(), stderr.String())
	}
}

// TestRunE2E_ClassDiagram runs the full CLI pipeline (flag parsing ->
// gocode.Parse -> diagram.Build -> mermaid.Render -> stdout) against
// each fixture and compares stdout against the very same
// expected-class.mmd golden files internal/render/mermaid's own tests
// use, verifying that CLI-level output matches renderer-level output
// exactly (including the trailing newline) and that nothing unwanted
// lands on stderr.
func TestRunE2E_ClassDiagram(t *testing.T) {
	cases := []string{"basic", "multi-package", "interfaces"}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := Run([]string{fixturesDir + "/" + name}, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty (no warnings expected for fixture %q)", stderr.String(), name)
			}

			testutil.Golden(t, fixturesDir+"/"+name+"/expected-class.mmd", stdout.String())
		})
	}
}

// TestRunE2E_EdgeCasesReportsWarning runs the CLI against the
// edge-cases fixture, whose broken.go is intentionally invalid Go: it
// must still exit 0 and produce the class diagram for everything else,
// while reporting broken.go as a warning on stderr rather than
// aborting.
func TestRunE2E_EdgeCasesReportsWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{fixturesDir + "/edge-cases"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "broken.go") {
		t.Errorf("stderr = %q, want it to mention broken.go", stderr.String())
	}

	testutil.Golden(t, fixturesDir+"/edge-cases/expected-class.mmd", stdout.String())
}

// TestRunE2E_IncludeExclude exercises --include/--exclude end to end:
// re-including *_test.go for the basic fixture must pull
// ShouldBeExcludedByDefault (declared in basic_test.go) into the
// output.
func TestRunE2E_IncludeExclude(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--exclude=*.md", fixturesDir + "/basic"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run exit code = %d, want 0 (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ShouldBeExcludedByDefault") {
		t.Errorf("stdout = %q, want it to include ShouldBeExcludedByDefault (default *_test.go exclusion replaced by --exclude=*.md)", stdout.String())
	}
}
