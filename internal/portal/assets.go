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
// small first-party driver script (portal.js), and the vendored
// mermaid.min.js/marked.min.js. writeStaticAssets copies these into
// every generated portal's assets/ directory. Vendored assets' sibling
// version.txt/LICENSE files are provenance records, not runtime
// dependencies, so they are intentionally not embedded here.
//
//go:embed assets/style.css assets/portal.js assets/vendor/mermaid.min.js assets/vendor/marked.min.js
var staticFS embed.FS
