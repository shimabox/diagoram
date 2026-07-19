// Package portal generates a self-contained, offline HTML portal from
// pre-rendered diagram/report/summary text. Rendering (choosing
// filters, calling internal/diagram and internal/render to produce
// Mermaid/PlantUML/summary/report text) is entirely the caller's
// responsibility; portal treats every artifact as an opaque string and
// only writes files, so it depends on neither internal/diagram nor
// internal/render and can be unit tested with fixed string input.
package portal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Artifacts holds every pre-rendered text artifact the portal embeds.
type Artifacts struct {
	// ClassMermaid and PackageMermaid are Mermaid classDiagram/flowchart
	// text, embedded as a text node and rendered in the browser.
	ClassMermaid, PackageMermaid string
	// ClassPlantUML and PackagePlantUML are PlantUML text, shown as
	// source only (the portal never renders PlantUML itself).
	ClassPlantUML, PackagePlantUML string
	// Summary is a plain-text structural summary.
	Summary string
	// ReportMarkdown is a Markdown analysis report, rendered
	// client-side with marked.min.js.
	ReportMarkdown string
}

// Meta holds portal-wide metadata shown in index.html's header.
type Meta struct {
	// Dir is the analyzed directory, as the caller displays it.
	Dir string
	// ModulePath is the detected Go module path, or "" if none.
	ModulePath string
	// Version is the diagoram version string.
	Version string
}

// Result reports where Generate wrote the portal's entry point.
type Result struct {
	// IndexPath is the generated index.html's path.
	IndexPath string
}

// maxMermaidBytes and maxMermaidEdges bound how large a Mermaid
// diagram Generate is willing to render in the browser. Beyond either
// limit, the corresponding page skips rendering (Mermaid's own
// flowchart maxEdges default is 500, and very large sources render
// slowly or illegibly) and falls back to a source-only view with a
// note on narrowing the analyzed set. maxMermaidBytes leaves headroom
// under the maxTextSize (900000) portal.js configures Mermaid with, so
// anything Generate lets through is also accepted by Mermaid itself.
const (
	maxMermaidBytes = 700000
	maxMermaidEdges = 500
)

// mermaidEdgePattern matches the arrow tokens diagoram's Mermaid
// renderers use for both classDiagram edges ("..>", "--|>", "..|>")
// and flowchart edges ("-->", "<-->"; the latter contains "-->" as a
// substring so no separate alternative is needed).
var mermaidEdgePattern = regexp.MustCompile(`-->|\.\.>|--\|>|\.\.\|>`)

// Generate writes a self-contained HTML portal describing a, annotated
// with meta, into outDir: index.html, one page per artifact, raw
// source copies (.mmd/.puml/.md/.txt), and the vendored/first-party
// assets that render them. outDir (and its assets/ subdirectory) is
// created if missing via os.MkdirAll; files Generate writes are
// overwritten in place, and Generate never deletes anything it did not
// itself write. Generate never embeds a generation timestamp, so
// repeated runs with identical input produce byte-identical output.
func Generate(outDir string, a Artifacts, meta Meta) (*Result, error) {
	if err := os.MkdirAll(filepath.Join(outDir, "assets", "vendor"), 0o755); err != nil {
		return nil, fmt.Errorf("portal: cannot create output directory %q: %w", outDir, err)
	}

	if err := writeStaticAssets(outDir); err != nil {
		return nil, err
	}

	sources := []struct{ name, content string }{
		{"class-diagram.mmd", a.ClassMermaid},
		{"package-diagram.mmd", a.PackageMermaid},
		{"class-diagram.puml", a.ClassPlantUML},
		{"package-diagram.puml", a.PackagePlantUML},
		{"report.md", a.ReportMarkdown},
		{"summary.txt", a.Summary},
	}
	for _, s := range sources {
		if err := writeSourceFile(outDir, s.name, s.content); err != nil {
			return nil, err
		}
	}

	pages := []func() error{
		func() error {
			return writeMermaidPage(outDir, "class-diagram.html", "Types and relationships", "class-diagram.mmd", a.ClassMermaid)
		},
		func() error {
			return writeMermaidPage(outDir, "package-diagram.html", "Package diagram", "package-diagram.mmd", a.PackageMermaid)
		},
		func() error {
			return writePumlPage(outDir, "class-diagram-puml.html", "Types and relationships (PlantUML source)", "class-diagram.puml", a.ClassPlantUML)
		},
		func() error {
			return writePumlPage(outDir, "package-diagram-puml.html", "Package diagram (PlantUML source)", "package-diagram.puml", a.PackagePlantUML)
		},
		func() error { return writeReportPage(outDir, a.ReportMarkdown) },
		func() error { return writeTextPage(outDir, "summary.html", "Summary", "summary.txt", a.Summary) },
		func() error { return writeIndexPage(outDir, meta) },
	}
	for _, p := range pages {
		if err := p(); err != nil {
			return nil, err
		}
	}

	return &Result{IndexPath: filepath.Join(outDir, "index.html")}, nil
}

// writeSourceFile writes content verbatim to <outDir>/name (the raw
// .mmd/.puml/.md/.txt copies alongside each rendered page).
func writeSourceFile(outDir, name, content string) error {
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("portal: cannot write %s: %w", name, err)
	}
	return nil
}

// writeStaticAssets copies every embedded runtime asset from staticFS
// into <outDir>/assets, preserving the vendor/ subdirectory.
func writeStaticAssets(outDir string) error {
	files := []string{
		"assets/style.css",
		"assets/portal.js",
		"assets/vendor/mermaid.min.js",
		"assets/vendor/marked.min.js",
	}
	for _, name := range files {
		data, err := staticFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("portal: cannot read embedded asset %s: %w", name, err)
		}
		dst := filepath.Join(outDir, filepath.FromSlash(name))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("portal: cannot write asset %s: %w", name, err)
		}
	}
	return nil
}

// mermaidTooLarge decides whether source is small enough for
// writeMermaidPage to attempt in-browser rendering. It reports true
// with a human-readable reason when source exceeds maxMermaidBytes or
// contains more than maxMermaidEdges edge lines, mirroring Mermaid's
// own flowchart maxEdges default (500) and the maxTextSize portal.js
// configures. This is the Go-side half of the two-tier maxEdges
// safeguard; portal.js's mermaid.parseError hook and run() rejection
// handler are the client-side half, catching sources that pass this
// check but still fail to render.
func mermaidTooLarge(source string) (skipped bool, reason string) {
	if source == "" {
		return false, ""
	}
	if len(source) > maxMermaidBytes {
		return true, fmt.Sprintf(
			"This diagram's source is %d bytes, over the %d-byte limit diagoram renders in the browser. Narrow the analyzed set with --exclude-dir, --include-dir, or --rel-target and regenerate, or view the source below.",
			len(source), maxMermaidBytes,
		)
	}
	if edges := countMermaidEdges(source); edges > maxMermaidEdges {
		return true, fmt.Sprintf(
			"This diagram has %d edges, over Mermaid's %d-edge default rendering limit. Narrow the analyzed set with --exclude-dir, --include-dir, or --rel-target and regenerate, or view the source below.",
			edges, maxMermaidEdges,
		)
	}
	return false, ""
}

// countMermaidEdges counts source's lines that contain a Mermaid edge
// arrow token, approximating the diagram's edge count without parsing
// Mermaid syntax (portal has no Mermaid parser of its own; it treats
// every artifact as an opaque string).
func countMermaidEdges(source string) int {
	count := 0
	for _, line := range strings.Split(source, "\n") {
		if mermaidEdgePattern.MatchString(line) {
			count++
		}
	}
	return count
}
