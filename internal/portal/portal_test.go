package portal

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/testutil"
)

// fixedArtifacts returns a small, fixed Artifacts value used across
// portal's tests. Its content is arbitrary but deterministic and
// includes a generics-style Mermaid label ("Widget[T any]" would be a
// realistic Go generic; here a plain field is enough to exercise every
// page) so golden comparisons are stable across runs.
func fixedArtifacts() Artifacts {
	return Artifacts{
		ClassMermaid: "classDiagram\n" +
			"    class Widget[\"Widget\"] {\n" +
			"        +Name string\n" +
			"        +Tags string[]\n" +
			"    }\n",
		PackageMermaid: "flowchart TD\n" +
			"    a[\"a\"]\n" +
			"    b[\"b\"]\n" +
			"    a --> b\n",
		ClassPlantUML: "@startuml class-diagram\n" +
			"    class Widget {\n" +
			"        +Name : string\n" +
			"    }\n" +
			"@enduml\n",
		PackagePlantUML: "@startuml package-diagram\n" +
			"    [a] --> [b]\n" +
			"@enduml\n",
		Summary: "diagoram: 1 packages, 1 structs\n\n" +
			".\n  Widget           (struct) fields=1\n",
		ReportMarkdown: "# Go source analysis report\n\n" +
			"## Types and relationships\n\n" +
			"```mermaid\n" +
			"classDiagram\n" +
			"    class Widget[\"Widget\"]\n" +
			"```\n",
	}
}

func fixedMeta() Meta {
	return Meta{Dir: "example", ModulePath: "example.com/widget", Version: "test"}
}

// wantFiles lists every file Generate must write, relative to outDir.
var wantFiles = []string{
	"index.html",
	"assets/style.css",
	"assets/portal.js",
	"assets/vendor/mermaid.min.js",
	"assets/vendor/marked.min.js",
	"assets/vendor/MERMAID-LICENSE",
	"assets/vendor/MARKED-LICENSE",
	"type-diagram.mmd",
	"type-diagram.html",
	"package-diagram.mmd",
	"package-diagram.html",
	"type-diagram.puml",
	"type-diagram-puml.html",
	"package-diagram.puml",
	"package-diagram-puml.html",
	"report.md",
	"report.html",
	"summary.txt",
	"summary.html",
}

func TestGenerate_WritesFileTree(t *testing.T) {
	outDir := t.TempDir()
	result, err := Generate(outDir, fixedArtifacts(), fixedMeta())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	wantIndex := filepath.Join(outDir, "index.html")
	if result.IndexPath != wantIndex {
		t.Errorf("Result.IndexPath = %q, want %q", result.IndexPath, wantIndex)
	}

	for _, name := range wantFiles {
		path := filepath.Join(outDir, filepath.FromSlash(name))
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("Generate() did not write %s: %v", name, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("Generate() wrote %s empty", name)
		}
	}
}

func TestGenerate_SourceFilesAreVerbatim(t *testing.T) {
	outDir := t.TempDir()
	a := fixedArtifacts()
	if _, err := Generate(outDir, a, fixedMeta()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	cases := []struct {
		name    string
		content string
	}{
		{"type-diagram.mmd", a.ClassMermaid},
		{"package-diagram.mmd", a.PackageMermaid},
		{"type-diagram.puml", a.ClassPlantUML},
		{"package-diagram.puml", a.PackagePlantUML},
		{"report.md", a.ReportMarkdown},
		{"summary.txt", a.Summary},
	}
	for _, c := range cases {
		got, err := os.ReadFile(filepath.Join(outDir, c.name))
		if err != nil {
			t.Errorf("cannot read %s: %v", c.name, err)
			continue
		}
		if string(got) != c.content {
			t.Errorf("%s content = %q, want %q", c.name, string(got), c.content)
		}
	}
}

func TestGenerate_IndexLinksToEveryPage(t *testing.T) {
	outDir := t.TempDir()
	if _, err := Generate(outDir, fixedArtifacts(), fixedMeta()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	index, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("cannot read index.html: %v", err)
	}

	linked := []string{
		"type-diagram.html",
		"package-diagram.html",
		"type-diagram-puml.html",
		"package-diagram-puml.html",
		"report.html",
		"summary.html",
	}
	for _, href := range linked {
		if !strings.Contains(string(index), `href="`+href+`"`) {
			t.Errorf("index.html does not link to %s", href)
		}
	}
}

func TestGenerate_Golden(t *testing.T) {
	outDir := t.TempDir()
	if _, err := Generate(outDir, fixedArtifacts(), fixedMeta()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	pages := []string{
		"index.html",
		"type-diagram.html",
		"package-diagram.html",
		"type-diagram-puml.html",
		"package-diagram-puml.html",
		"report.html",
		"summary.html",
	}
	for _, name := range pages {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("cannot read %s: %v", name, err)
		}
		testutil.Golden(t, "testdata/golden/"+name, string(got))
	}
}

// TestGenerate_NoExternalURLs is the portal's core offline-completeness
// guarantee: no generated HTML page may reference an external
// http(s):// URL (analyzed source, warnings, and doc comments must
// never be able to smuggle a request off the machine, and every asset
// the pages load must be vendored). It intentionally does not scan
// assets/vendor/*.js: those are third-party files whose license
// headers/source-map comments legitimately mention URLs; what matters
// is that the portal's own generated pages never reference one.
func TestGenerate_NoExternalURLs(t *testing.T) {
	outDir := t.TempDir()
	if _, err := Generate(outDir, fixedArtifacts(), fixedMeta()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), "http://") || strings.Contains(string(content), "https://") {
			t.Errorf("%s contains an external URL reference", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
}

func TestGenerate_OverwritesWithoutDeletingUnrelatedFiles(t *testing.T) {
	outDir := t.TempDir()
	unrelated := filepath.Join(outDir, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("keep me\n"), 0o644); err != nil {
		t.Fatalf("cannot seed unrelated file: %v", err)
	}

	if _, err := Generate(outDir, fixedArtifacts(), fixedMeta()); err != nil {
		t.Fatalf("first Generate() error = %v", err)
	}
	// A second, slightly different run must overwrite in place rather
	// than failing or leaving stale content.
	a2 := fixedArtifacts()
	a2.Summary = "diagoram: 2 packages, 2 structs\n"
	if _, err := Generate(outDir, a2, fixedMeta()); err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}

	got, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatalf("Generate() deleted an unrelated file: %v", err)
	}
	if string(got) != "keep me\n" {
		t.Errorf("unrelated file content changed: %q", string(got))
	}

	summary, err := os.ReadFile(filepath.Join(outDir, "summary.txt"))
	if err != nil {
		t.Fatalf("cannot read summary.txt: %v", err)
	}
	if string(summary) != a2.Summary {
		t.Errorf("summary.txt = %q, want the second run's content %q", string(summary), a2.Summary)
	}
}

func TestMermaidTooLarge(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantSkip   bool
		wantReason string // substring
	}{
		{
			name:     "empty",
			source:   "",
			wantSkip: false,
		},
		{
			name:     "small diagram",
			source:   "classDiagram\n    class A\n    A --> B\n",
			wantSkip: false,
		},
		{
			name:       "too many bytes",
			source:     strings.Repeat("x", maxMermaidBytes+1),
			wantSkip:   true,
			wantReason: "bytes",
		},
		{
			name:       "too many edges",
			source:     "flowchart TD\n" + strings.Repeat("    a --> b\n", maxMermaidEdges+1),
			wantSkip:   true,
			wantReason: "edges",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipped, reason := mermaidTooLarge(tt.source)
			if skipped != tt.wantSkip {
				t.Errorf("mermaidTooLarge() skipped = %v, want %v (reason=%q)", skipped, tt.wantSkip, reason)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Errorf("mermaidTooLarge() reason = %q, want it to contain %q", reason, tt.wantReason)
			}
			if !tt.wantSkip && reason != "" {
				t.Errorf("mermaidTooLarge() reason = %q, want empty when not skipped", reason)
			}
		})
	}
}

func TestCountMermaidEdges(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{"none", "classDiagram\n    class A\n", 0},
		{"dependency arrows", "classDiagram\n    A ..> B\n    B ..> C\n", 2},
		{"mutual flowchart arrow counts once", "flowchart TD\n    a <--> b\n", 1},
		{"mixed classDiagram arrows", "classDiagram\n    A --|> B\n    A ..|> C\n    A ..> D\n", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countMermaidEdges(tt.source); got != tt.want {
				t.Errorf("countMermaidEdges() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestGenerate_MermaidFallbackForLargeDiagram is portal's end-to-end
// check for the maxEdges branch: a package diagram over the edge
// threshold must fall back to a source-only page (no live "mermaid"
// class, a skip notice, and the raw source still present/downloadable)
// while an unrelated, small diagram on the same run renders normally.
func TestGenerate_MermaidFallbackForLargeDiagram(t *testing.T) {
	outDir := t.TempDir()
	a := fixedArtifacts()

	var big strings.Builder
	big.WriteString("flowchart TD\n")
	for i := 0; i < maxMermaidEdges+1; i++ {
		big.WriteString("    n" + strconv.Itoa(i) + " --> n" + strconv.Itoa(i+1) + "\n")
	}
	a.PackageMermaid = big.String()

	if _, err := Generate(outDir, a, fixedMeta()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	packagePage, err := os.ReadFile(filepath.Join(outDir, "package-diagram.html"))
	if err != nil {
		t.Fatalf("cannot read package-diagram.html: %v", err)
	}
	if strings.Contains(string(packagePage), `class="mermaid"`) {
		t.Errorf("package-diagram.html still attempts live rendering of an oversized diagram")
	}
	if !strings.Contains(string(packagePage), "too large to render reliably") &&
		!strings.Contains(string(packagePage), "edges, over Mermaid") {
		t.Errorf("package-diagram.html does not explain why rendering was skipped:\n%s", packagePage)
	}
	if !strings.Contains(string(packagePage), "n0 --&gt; n1") && !strings.Contains(string(packagePage), "n0 --> n1") {
		t.Errorf("package-diagram.html dropped the source it skipped rendering")
	}

	classPage, err := os.ReadFile(filepath.Join(outDir, "type-diagram.html"))
	if err != nil {
		t.Fatalf("cannot read type-diagram.html: %v", err)
	}
	if !strings.Contains(string(classPage), `class="mermaid"`) {
		t.Errorf("type-diagram.html should still render live; the fallback must be per-diagram, not global")
	}
}
