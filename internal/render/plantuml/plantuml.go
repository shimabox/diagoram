// Package plantuml renders a diagram.Diagram or diagram.PackageGraph
// as PlantUML script (https://plantuml.com/class-diagram,
// https://plantuml.com/deployment-diagram for the package-nesting
// syntax it borrows). It implements render.Renderer and
// render.PackageGraphRenderer, the same interfaces
// internal/render/mermaid implements, from the same intermediate
// representation.
package plantuml

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
)

// indentUnit is one level of indentation, per diagoram's output
// convention (spaces, not tabs) — the same convention
// internal/render/mermaid uses.
const indentUnit = "    "

// Renderer renders a diagram.Diagram as PlantUML class-diagram script,
// and a diagram.PackageGraph as PlantUML package-diagram script (see
// package.go). It implements render.Renderer and
// render.PackageGraphRenderer.
type Renderer struct{}

// New returns a PlantUML Renderer.
func New() *Renderer {
	return &Renderer{}
}

// Render returns d as a PlantUML class diagram: an "@startuml
// class-diagram" / "@enduml" pair wrapping one "package ... as ... {
// }" block per non-root package that declares at least one Entry
// (root-level Entries, if any, are rendered directly with no package
// wrapper, exactly mirroring how internal/render/mermaid's Render
// leaves them un-namespaced), nested to match the true package
// hierarchy (unlike Mermaid, PlantUML packages nest freely), and
// finally every Edge as an arrow line. opt controls which members and
// edges are included (see render.Options). Render never returns a
// non-nil error; it is declared to return one to satisfy
// render.Renderer and to leave room for future validation.
func (r *Renderer) Render(d *diagram.Diagram, opt render.Options) (string, error) {
	lines := []string{"@startuml class-diagram"}
	lines = append(lines, renderTree(d.Root, 1, opt)...)
	lines = append(lines, renderEdges(d.Edges, opt)...)
	lines = append(lines, "@enduml")
	return strings.Join(lines, "\n") + "\n", nil
}

// renderTree renders node's own Entries followed by one nested
// "package" block per child that owns an Entry or has its own
// nested children, at the given indentation depth.
func renderTree(node *diagram.PackageNode, depth int, opt render.Options) []string {
	var lines []string
	for _, e := range node.Entries {
		lines = append(lines, renderClass(e, depth, opt)...)
	}
	for _, c := range node.Children {
		lines = append(lines, renderPackageBlock(c, depth, opt)...)
	}
	return lines
}

// renderPackageBlock renders node as a "package Name as alias { ...
// }" block containing its own Entries and, recursively, its
// Children's own blocks — PlantUML packages nest freely, so a child
// directory's block is a true child of its parent's, unlike Mermaid's
// flattened namespaces.
//
// A node whose entire subtree owns no Entry at all (e.g. a package
// directory that declares only functions, not structs/interfaces, and
// has no nested package that declares any either — diagoram's own
// cmd/diagoram is one such case) is omitted entirely, matching
// internal/render/mermaid's own renderSubtree, which likewise never
// opens a namespace block with nothing in it. Unlike renderSubtree,
// this check is on the whole subtree (via the recursive renderTree
// call below), not just node's own Entries: an ancestor directory with
// no Entries of its own but a descendant that does still needs its
// block, to keep the nesting PlantUML draws accurate.
func renderPackageBlock(node *diagram.PackageNode, depth int, opt render.Options) []string {
	inner := renderTree(node, depth+1, opt)
	if len(inner) == 0 {
		return nil
	}
	indent := strings.Repeat(indentUnit, depth)
	lines := []string{fmt.Sprintf("%spackage %s as %s {", indent, node.Name, packageAlias(node.Path))}
	lines = append(lines, inner...)
	lines = append(lines, indent+"}")
	return lines
}

// renderClass renders a single Entry at the given indentation depth,
// applying opt's display filters (--hide-unexported, --disable-fields,
// --disable-methods) to its member lists first.
//
// The PlantUML keyword itself ("class" or "interface") already says
// what kind of Entry this is, so — unlike internal/render/mermaid,
// which must always open a body block for an interface to carry its
// "<<interface>>" stereotype line — renderClass collapses an Entry
// with no visible fields or methods to a single line with no "{ }"
// block regardless of Kind (matching php-class-diagram's own
// convention for a body-less entry; see Entry.php's dump()).
//
// When e.Doc is non-empty, it is appended to the quoted display name
// as a second, bold line (e.g. `"Product\n<b>Product represents an
// item for sale.</b>"`), PlantUML's own line-break escape inside a
// quoted string — the same visual effect as php-class-diagram's
// --enable-class-name-summary option. Mermaid has no safe place to put
// this free-text summary inside a class body, which is exactly why
// Phase 3 deferred it to the PlantUML renderer (see Entry.Doc's doc
// comment).
func renderClass(e *diagram.Entry, depth int, opt render.Options) []string {
	indent := strings.Repeat(indentUnit, depth)
	memberIndent := strings.Repeat(indentUnit, depth+1)

	keyword := "class"
	if e.Kind == diagram.KindInterface {
		keyword = "interface"
	}
	header := fmt.Sprintf(`%s%s "%s%s" as %s`, indent, keyword, e.Name, docSummary(e.Doc), e.ID)

	fields, methods := visibleMembers(e, opt)
	if len(fields) == 0 && len(methods) == 0 {
		return []string{header}
	}

	lines := []string{header + " {"}
	for _, f := range fields {
		lines = append(lines, memberIndent+fieldLine(f))
	}
	for _, m := range methods {
		lines = append(lines, memberIndent+methodLine(m))
	}
	lines = append(lines, indent+"}")
	return lines
}

// visibleMembers returns e's fields and methods after applying opt:
// --disable-fields/--disable-methods drop a member list outright, and
// --hide-unexported (applied afterward) drops unexported members from
// whatever remains. It mirrors internal/render/mermaid's own
// visibleMembers; the two are not shared because doing so would need
// either a shared subpackage or an import between the two renderers
// for a handful of lines with no other relationship between the
// formats.
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

// fieldLine renders one field as "[+-]Name : Type".
func fieldLine(f gocode.Field) string {
	return visibility(f.Exported) + f.Name + " : " + safeType(f.Type.String)
}

// methodLine renders one method as "[+-]Name(ParamType, ...) :
// ResultType, ...", omitting the trailing " : ResultType, ..." part
// entirely when the method has no results.
func methodLine(m gocode.Method) string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = safeType(p.String)
	}
	results := make([]string, len(m.Results))
	for i, res := range m.Results {
		results[i] = safeType(res.String)
	}

	line := visibility(m.Exported) + m.Name + "(" + strings.Join(params, ", ") + ")"
	if len(results) > 0 {
		line += " : " + strings.Join(results, ", ")
	}
	return line
}

// visibility returns PlantUML's "+"/"-" member-visibility marker.
func visibility(exported bool) string {
	if exported {
		return "+"
	}
	return "-"
}

// docSummary renders e's doc comment (already just its first line; see
// diagram.Entry.Doc) as a PlantUML line-break escape ("\n", the two
// literal characters backslash-n, which PlantUML interprets as a
// forced line break inside a quoted string) followed by a Creole
// bold-text span, ready to append directly after a class's quoted
// display name. It returns "" when doc is empty, so a class with no
// doc comment gets no second line at all.
func docSummary(doc string) string {
	if doc == "" {
		return ""
	}
	return `\n<b>` + escapeQuoted(doc) + `</b>`
}

// escapeQuoted escapes s for embedding inside a PlantUML double-quoted
// string: a literal backslash must not be read as the start of an
// escape sequence (e.g. PlantUML's own "\n" line-break escape), and a
// literal double quote must not end the string early.
func escapeQuoted(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// renderEdges renders every Edge as an arrow line:
//
//   - Dependency edges use "..>", or "\"1\" ..> \"*\"" when
//     e.ToCollection is set, as a multiplicity hint for a slice/array/map
//     reference — the same convention php-class-diagram uses for a
//     collection-typed dependency (see ArrowDependency.php).
//   - Embedding edges are written "To <|-- From" (the embedded/parent
//     type on the left, using PlantUML's left-pointing inheritance
//     arrow) rather than "From --|> To": this is php-class-diagram's own
//     convention for its equivalent "extends" arrow (see
//     ArrowInheritance.php), and following it here keeps diagoram's
//     PlantUML output idiomatic for readers already familiar with that
//     tool.
//   - Implementation edges use "..|>", from the implementing struct to
//     the interface (skipped entirely when opt.DisableImplements is
//     set), matching both Mermaid's own direction and the phase plan's
//     worked example.
func renderEdges(edges []diagram.Edge, opt render.Options) []string {
	lines := make([]string, 0, len(edges))
	for _, e := range edges {
		if e.Kind == diagram.Implementation && opt.DisableImplements {
			continue
		}

		var line string
		switch e.Kind {
		case diagram.Embedding:
			line = e.To + " <|-- " + e.From
		case diagram.Implementation:
			line = e.From + " ..|> " + e.To
		default: // diagram.Dependency
			if e.ToCollection {
				line = fmt.Sprintf(`%s "1" ..> "*" %s`, e.From, e.To)
			} else {
				line = e.From + " ..> " + e.To
			}
		}
		lines = append(lines, indentUnit+line)
	}
	return lines
}

// unsafeAliasChar matches any rune that is not safe to use in a
// PlantUML package alias.
var unsafeAliasChar = regexp.MustCompile(`[^A-Za-z0-9_]`)

// packageAlias turns a PackageNode.Path (e.g. "product/attribute")
// into a flat, identifier-safe PlantUML package alias (e.g.
// "product_attribute"), reusing the same flattening convention
// internal/render/mermaid's namespaceName uses for the equivalent
// purpose, so a package's alias always matches the ID prefix diagoram
// already gives every Entry declared in it (e.g. class alias
// "product_attribute_Color" sits inside "package attribute as
// product_attribute").
func packageAlias(path string) string {
	return unsafeAliasChar.ReplaceAllString(path, "_")
}
