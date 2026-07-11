package diagram

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes contents to filepath.Join(dir, name), creating any
// missing parent directories, and fails t if it cannot.
func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestReadModulePath(t *testing.T) {
	tests := []struct {
		name    string
		goMod   string // "" means: do not create a go.mod at all
		want    string
		wantErr bool
	}{
		{
			name:  "plain module directive",
			goMod: "module github.com/shimabox/diagoram\n\ngo 1.21\n",
			want:  "github.com/shimabox/diagoram",
		},
		{
			name:  "fixture module path",
			goMod: "module example.com/looptest\n\ngo 1.21\n",
			want:  "example.com/looptest",
		},
		{
			name:  "module directive not on the first line",
			goMod: "// some doc comment\n\nmodule example.com/mid\n\ngo 1.22\n\nrequire foo v1.0.0\n",
			want:  "example.com/mid",
		},
		{
			name:  "trailing line comment on the module line",
			goMod: "module example.com/commented // keep me honest\n\ngo 1.21\n",
			want:  "example.com/commented",
		},
		{
			name:  "quoted module path",
			goMod: "module \"example.com/quoted\"\n\ngo 1.21\n",
			want:  "example.com/quoted",
		},
		{
			name:  "CRLF line endings",
			goMod: "module example.com/crlf\r\n\r\ngo 1.21\r\n",
			want:  "example.com/crlf",
		},
		{
			name:  "no go.mod at all",
			goMod: "",
			want:  "",
		},
		{
			name:  "go.mod with no module directive",
			goMod: "go 1.21\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.goMod != "" {
				writeFile(t, dir, "go.mod", tt.goMod)
			}

			got, err := ReadModulePath(dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadModulePath(%q) error = %v, wantErr %v", dir, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ReadModulePath(%q) = %q, want %q", dir, got, tt.want)
			}
		})
	}
}

// TestReadModulePath_UnreadableGoMod makes sure a go.mod that exists
// but cannot be read (as opposed to one that does not exist) is
// reported as an error rather than silently treated as "no module".
func TestReadModulePath_UnreadableGoMod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	writeFile(t, dir, "go.mod", "module example.com/unreadable\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not block reads")
	}

	if _, err := ReadModulePath(dir); err == nil {
		t.Errorf("ReadModulePath(%q) error = nil, want a permission error", dir)
	}
}
