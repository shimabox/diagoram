package portal

import (
	"embed"
	"html/template"
)

// templateFS embeds every page template. Template names, as parsed by
// pageTemplates, are their base file names (e.g. "index.html.tmpl").
//
//go:embed templates/*.html.tmpl
var templateFS embed.FS

// pageTemplates is parsed once at package init from templateFS.
var pageTemplates = template.Must(template.ParseFS(templateFS, "templates/*.html.tmpl"))

// staticFS embeds the portal's runtime assets: the stylesheet, the
// small first-party driver script (portal.js), the vendored
// mermaid.min.js/marked.min.js, and their MIT license texts.
// writeStaticAssets copies these into every generated portal's assets/
// directory. The LICENSE files must ship alongside the vendored
// scripts so generated portals (which are shared/published, e.g. to
// GitHub Pages) carry the copyright notice and permission text MIT
// requires; the sibling version.txt files are provenance records, not
// redistribution requirements, so they remain intentionally
// unembedded.
//
//go:embed assets/style.css assets/portal.js assets/vendor/mermaid.min.js assets/vendor/marked.min.js assets/vendor/MERMAID-LICENSE assets/vendor/MARKED-LICENSE
var staticFS embed.FS
