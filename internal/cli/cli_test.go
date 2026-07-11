package cli

import (
	"bytes"
	"strings"
	"testing"
)

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
