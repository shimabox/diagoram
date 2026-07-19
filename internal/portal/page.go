package portal

import (
	"fmt"
	"os"
	"path/filepath"
)

// indexPageData is index.html.tmpl's template data.
type indexPageData struct {
	Dir, ModulePath, Version string
}

// mermaidPageData is mermaid.html.tmpl's template data. When Skipped
// is true, SkipReason explains why (see mermaidTooLarge) and Source
// still holds the raw text so it can be shown/copied, just not
// rendered.
type mermaidPageData struct {
	Title, SourceFile, Source string
	Skipped                   bool
	SkipReason                string
}

// pumlPageData is puml.html.tmpl's template data.
type pumlPageData struct {
	Title, SourceFile, Source string
}

// textPageData is text.html.tmpl's template data.
type textPageData struct {
	Title, SourceFile, Content string
}

// reportPageData is report.html.tmpl's template data.
type reportPageData struct {
	Title, SourceFile, Markdown string
}

// writeIndexPage renders index.html.tmpl to <outDir>/index.html.
func writeIndexPage(outDir string, meta Meta) error {
	return renderPage(outDir, "index.html", "index.html.tmpl", indexPageData{
		Dir:        meta.Dir,
		ModulePath: meta.ModulePath,
		Version:    meta.Version,
	})
}

// writeMermaidPage renders mermaid.html.tmpl to <outDir>/<filename>,
// applying the maxEdges/maxTextSize precheck (mermaidTooLarge) to
// decide whether the page attempts in-browser rendering or falls back
// to a source-only view.
func writeMermaidPage(outDir, filename, title, sourceFile, source string) error {
	skipped, reason := mermaidTooLarge(source)
	return renderPage(outDir, filename, "mermaid.html.tmpl", mermaidPageData{
		Title:      title,
		SourceFile: sourceFile,
		Source:     source,
		Skipped:    skipped,
		SkipReason: reason,
	})
}

// writePumlPage renders puml.html.tmpl to <outDir>/<filename>.
func writePumlPage(outDir, filename, title, sourceFile, source string) error {
	return renderPage(outDir, filename, "puml.html.tmpl", pumlPageData{
		Title:      title,
		SourceFile: sourceFile,
		Source:     source,
	})
}

// writeTextPage renders text.html.tmpl to <outDir>/<filename>.
func writeTextPage(outDir, filename, title, sourceFile, content string) error {
	return renderPage(outDir, filename, "text.html.tmpl", textPageData{
		Title:      title,
		SourceFile: sourceFile,
		Content:    content,
	})
}

// writeReportPage renders report.html.tmpl to <outDir>/report.html.
func writeReportPage(outDir, markdown string) error {
	return renderPage(outDir, "report.html", "report.html.tmpl", reportPageData{
		Title:      "Report",
		SourceFile: "report.md",
		Markdown:   markdown,
	})
}

// renderPage executes the named template from pageTemplates with data
// and writes the result to <outDir>/<filename>, overwriting any
// existing file there.
func renderPage(outDir, filename, templateName string, data any) error {
	path := filepath.Join(outDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("portal: cannot create %s: %w", filename, err)
	}
	defer f.Close()

	if err := pageTemplates.ExecuteTemplate(f, templateName, data); err != nil {
		return fmt.Errorf("portal: cannot render %s: %w", filename, err)
	}
	return nil
}
