// Package mermaid renders a diagram.Diagram as Mermaid classDiagram
// text (https://mermaid.js.org/syntax/classDiagram.html).
package mermaid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
)

// indentUnit is one level of indentation, per diagoram's output
// convention (spaces, not tabs).
const indentUnit = "    "

// Renderer renders a diagram.Diagram as Mermaid classDiagram text. It
// implements render.Renderer.
type Renderer struct{}

// New returns a Mermaid Renderer.
func New() *Renderer {
	return &Renderer{}
}

// Render returns d as Mermaid classDiagram text: a "classDiagram"
// header, one flattened `namespace` block per non-root package that
// declares at least one Entry (root-level Entries, if any, are
// rendered directly with no namespace), and finally every Edge as an
// arrow line. Render never returns a non-nil error; it is declared to
// return one to satisfy render.Renderer and to leave room for future
// validation.
func (r *Renderer) Render(d *diagram.Diagram, _ render.Options) (string, error) {
	lines := []string{"classDiagram"}
	lines = append(lines, renderTree(d.Root)...)
	lines = append(lines, renderEdges(d.Edges)...)
	return strings.Join(lines, "\n") + "\n", nil
}

// renderTree renders root's own Entries (un-namespaced, since root
// has no meaningful package path) followed by one namespace block per
// descendant node that owns Entries.
func renderTree(root *diagram.PackageNode) []string {
	var lines []string
	for _, e := range root.Entries {
		lines = append(lines, renderClass(e, 1)...)
	}
	for _, c := range root.Children {
		lines = append(lines, renderSubtree(c)...)
	}
	return lines
}

// renderSubtree renders node as a flat namespace block (if it owns
// any Entries) and recurses into its children. Mermaid namespaces
// cannot nest, so a deeper node's block is a sibling of its parent's,
// not nested inside it; namespaceName encodes the full package path so
// sibling blocks never collide.
func renderSubtree(node *diagram.PackageNode) []string {
	var lines []string
	if len(node.Entries) > 0 {
		lines = append(lines, indentUnit+"namespace "+namespaceName(node.Path)+" {")
		for _, e := range node.Entries {
			lines = append(lines, renderClass(e, 2)...)
		}
		lines = append(lines, indentUnit+"}")
	}
	for _, c := range node.Children {
		lines = append(lines, renderSubtree(c)...)
	}
	return lines
}

// renderClass renders a single Entry at the given indentation depth
// (1 = top-level, 2 = inside one namespace block). Entries with no
// body content (a struct with no fields or methods) are rendered as a
// single line with no "{ }" block, matching diagoram's golden fixture
// convention; interfaces always get a block, since it must carry the
// <<interface>> stereotype line.
func renderClass(e *diagram.Entry, depth int) []string {
	indent := strings.Repeat(indentUnit, depth)
	memberIndent := strings.Repeat(indentUnit, depth+1)
	header := fmt.Sprintf(`%sclass %s["%s"]`, indent, e.ID, e.Name)

	hasBody := e.Kind == diagram.KindInterface || len(e.Fields) > 0 || len(e.Methods) > 0
	if !hasBody {
		return []string{header}
	}

	lines := []string{header + " {"}
	if e.Kind == diagram.KindInterface {
		lines = append(lines, memberIndent+"<<interface>>")
	}
	for _, f := range e.Fields {
		lines = append(lines, memberIndent+fieldLine(f))
	}
	for _, m := range e.Methods {
		lines = append(lines, memberIndent+methodLine(m))
	}
	lines = append(lines, indent+"}")
	return lines
}

// fieldLine renders one field as "[+-]Name Type".
func fieldLine(f gocode.Field) string {
	return visibility(f.Exported) + f.Name + " " + formatType(f.Type.String)
}

// methodLine renders one method as "[+-]Name(ParamType, ...)
// ResultType, ...", omitting the trailing result list entirely when
// the method has no results.
func methodLine(m gocode.Method) string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = formatType(p.String)
	}
	results := make([]string, len(m.Results))
	for i, res := range m.Results {
		results[i] = formatType(res.String)
	}

	line := visibility(m.Exported) + m.Name + "(" + strings.Join(params, ", ") + ")"
	if len(results) > 0 {
		line += " " + strings.Join(results, ", ")
	}
	return line
}

// visibility returns Mermaid's "+"/"-" member-visibility marker.
func visibility(exported bool) string {
	if exported {
		return "+"
	}
	return "-"
}

// renderEdges renders every Edge as an arrow line: Dependency edges
// use "..>" (optionally suffixed with a " : *" label when the
// underlying reference was to a slice/array or a map, as a stand-in
// for multiplicity), and Embedding edges use "--|>".
func renderEdges(edges []diagram.Edge) []string {
	lines := make([]string, 0, len(edges))
	for _, e := range edges {
		arrow := "..>"
		if e.Kind == diagram.Embedding {
			arrow = "--|>"
		}
		line := indentUnit + e.From + " " + arrow + " " + e.To
		if e.Kind == diagram.Dependency && e.ToCollection {
			line += ` : *`
		}
		lines = append(lines, line)
	}
	return lines
}

// unsafeNamespaceChar matches any rune that is not safe to use in a
// Mermaid namespace identifier.
var unsafeNamespaceChar = regexp.MustCompile(`[^A-Za-z0-9_]`)

// namespaceName turns a PackageNode.Path (e.g. "product/attribute")
// into a flat, identifier-safe Mermaid namespace name (e.g.
// "product_attribute"), mirroring the Entry.ID prefix diagoram already
// uses for that same package, so classes visually nest under the
// namespace whose name matches their own ID prefix.
func namespaceName(path string) string {
	return unsafeNamespaceChar.ReplaceAllString(path, "_")
}
