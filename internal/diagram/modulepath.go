package diagram

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadModulePath reads the "module" directive from rootDir/go.mod, if
// one exists, and returns the module path it declares (e.g.
// "github.com/shimabox/diagoram"). Only the module line's text is
// interpreted — go.mod is never parsed as a dependency/build graph
// and rootDir is never built — so this works even when the module's
// dependencies are unavailable or the code does not otherwise build.
//
// ReadModulePath returns ("", nil), not an error, both when rootDir
// has no go.mod and when a go.mod exists but has no module directive
// diagoram can make sense of: callers fall back to approximating
// import-path-to-package-directory resolution instead (see
// resolveImportDir). A non-nil error is only returned for problems
// unrelated to whether a module path is declared, such as go.mod
// existing but being unreadable.
func ReadModulePath(rootDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return parseModulePath(string(data)), nil
}

// parseModulePath scans a go.mod file's contents line by line for its
// "module" directive and returns the module path it declares, or ""
// if no such directive is found. The module path may be written bare
// (module example.com/foo) or as a quoted Go string literal (module
// "example.com/foo"); a trailing "//" line comment on the module line
// is ignored.
func parseModulePath(contents string) string {
	for _, line := range strings.Split(contents, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))

		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest = strings.TrimSpace(rest)

		if idx := strings.Index(rest, "//"); idx >= 0 {
			rest = strings.TrimSpace(rest[:idx])
		}
		if rest == "" {
			continue
		}

		if unquoted, err := strconv.Unquote(rest); err == nil {
			return unquoted
		}
		return rest
	}
	return ""
}
