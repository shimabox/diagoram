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
// arrow line. opt controls which members and edges are included (see
// render.Options). Render never returns a non-nil error; it is
// declared to return one to satisfy render.Renderer and to leave room
// for future validation.
func (r *Renderer) Render(d *diagram.Diagram, opt render.Options) (string, error) {
	if opt.HideUnexported {
		d = diagram.FilterUnexported(d)
	}
	lines := []string{"classDiagram"}
	lines = append(lines, renderTree(d.Root, opt)...)
	lines = append(lines, renderEdges(d.Edges, opt)...)
	return strings.Join(lines, "\n") + "\n", nil
}

// renderTree renders root's own Entries (un-namespaced, since root
// has no meaningful package path) followed by one namespace block per
// descendant node that owns Entries.
func renderTree(root *diagram.PackageNode, opt render.Options) []string {
	var lines []string
	for _, e := range root.Entries {
		lines = append(lines, renderClass(e, 1, opt)...)
	}
	for _, c := range root.Children {
		lines = append(lines, renderSubtree(c, opt)...)
	}
	return lines
}

// renderSubtree renders node as a flat namespace block (if it owns
// any Entries) and recurses into its children. Mermaid namespaces
// cannot nest, so a deeper node's block is a sibling of its parent's,
// not nested inside it; namespaceName encodes the full package path so
// sibling blocks never collide.
func renderSubtree(node *diagram.PackageNode, opt render.Options) []string {
	var lines []string
	if len(node.Entries) > 0 {
		lines = append(lines, indentUnit+"namespace "+namespaceName(node.Path)+" {")
		for _, e := range node.Entries {
			lines = append(lines, renderClass(e, 2, opt)...)
		}
		lines = append(lines, indentUnit+"}")
	}
	for _, c := range node.Children {
		lines = append(lines, renderSubtree(c, opt)...)
	}
	return lines
}

// renderClass renders a single Entry at the given indentation depth
// (1 = top-level, 2 = inside one namespace block), applying opt's
// display filters (--hide-unexported, --disable-fields,
// --disable-methods) to its member lists first. Entries with no body
// content left to show (e.g. a struct with no fields or methods once
// filtered) are rendered as a single line with no "{ }" block, matching
// diagoram's golden fixture convention; interfaces always get a block,
// since it must carry the <<interface>> stereotype line.
func renderClass(e *diagram.Entry, depth int, opt render.Options) []string {
	indent := strings.Repeat(indentUnit, depth)
	memberIndent := strings.Repeat(indentUnit, depth+1)
	header := fmt.Sprintf(`%sclass %s["%s"]`, indent, e.ID, e.Name)

	fields, methods := visibleMembers(e, opt)

	hasBody := e.Kind == diagram.KindInterface || e.Kind == diagram.KindNamedType || len(fields) > 0 || len(methods) > 0
	if !hasBody {
		return []string{header}
	}

	lines := []string{header + " {"}
	if e.Kind == diagram.KindInterface {
		lines = append(lines, memberIndent+"<<interface>>")
	}
	if e.Kind == diagram.KindNamedType {
		lines = append(lines, memberIndent+"<<"+diagram.NamedTypeLabel(e.NamedType)+">>")
		if e.NamedType.Kind == gocode.NamedFunc {
			if len(e.NamedType.Params) > 0 {
				lines = append(lines, memberIndent+"params "+formattedTypeRefs(e.NamedType.Params))
			}
			if len(e.NamedType.Results) > 0 {
				lines = append(lines, memberIndent+"returns "+formattedTypeRefs(e.NamedType.Results))
			}
		} else {
			lines = append(lines, memberIndent+"type "+formatType(e.NamedType.Underlying.String))
		}
		if opt.ShowConstants {
			constants := e.NamedType.Constants
			if opt.HideUnexported {
				constants = diagram.ExportedConstants(constants)
			}
			for _, constant := range constants {
				lines = append(lines, memberIndent+visibility(constant.Exported)+constant.Name)
			}
		}
	}
	for _, f := range fields {
		lines = append(lines, memberIndent+fieldLine(f))
	}
	for _, m := range methods {
		lines = append(lines, memberIndent+methodLine(m))
	}
	lines = append(lines, indent+"}")
	return lines
}

func formattedTypeRefs(refs []gocode.TypeRef) string {
	formatted := make([]string, len(refs))
	for i, ref := range refs {
		formatted[i] = formatType(ref.String)
	}
	return strings.Join(formatted, ", ")
}

// visibleMembers returns e's fields and methods after applying opt:
// --disable-fields/--disable-methods drop a member list outright, and
// --hide-unexported (applied afterward) drops unexported members from
// whatever remains.
func visibleMembers(e *diagram.Entry, opt render.Options) ([]gocode.Field, []gocode.Method) {
	var fields []gocode.Field
	if !opt.DisableFields {
		fields = e.Fields
		if opt.HideUnexported {
			fields = diagram.ExportedFields(fields)
		}
	}
	var methods []gocode.Method
	if !opt.DisableMethods {
		methods = e.Methods
		if opt.HideUnexported {
			methods = diagram.ExportedMethods(methods)
		}
	}
	return fields, methods
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
// for multiplicity), Embedding edges use "--|>", and Implementation
// edges use "..|>" (skipped entirely when opt.DisableImplements is
// set).
func renderEdges(edges []diagram.Edge, opt render.Options) []string {
	lines := make([]string, 0, len(edges))
	for _, e := range edges {
		if e.Kind == diagram.Implementation && opt.DisableImplements {
			continue
		}
		arrow := "..>"
		switch e.Kind {
		case diagram.Embedding:
			arrow = "--|>"
		case diagram.Implementation:
			arrow = "..|>"
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
