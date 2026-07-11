package plantuml

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/render"
)

// RenderPackageGraph returns g as a PlantUML package diagram: an
// "@startuml package-related-diagram" / "@enduml" pair wrapping one
// "package Name as alias { ... }" block per analyzed directory that
// has its own child directories (nested to match the true directory
// hierarchy — PlantUML packages, unlike Mermaid subgraphs, nest
// freely) or, for a childless (leaf) directory, a single-line "package
// Name as alias" declaration with no body, followed by one
// light-colored external-package declaration per distinct external
// import path referenced (only ever present when g was built with
// showExternal = true), and finally every PackageEdge as an arrow
// line, with a Mutual edge drawn as a red, bold, bidirectional arrow
// (php-class-diagram's own convention for a direct two-package import
// cycle; see PackageArrow.php). RenderPackageGraph never returns a
// non-nil error; it is declared to return one to match Render's shape
// and to leave room for future validation.
func (r *Renderer) RenderPackageGraph(g *diagram.PackageGraph, _ render.Options) (string, error) {
	lines := []string{"@startuml package-related-diagram"}

	if g.HasRootPackage {
		lines = append(lines, indentUnit+`package "." as root`)
	}
	lines = append(lines, renderPackageTree(g.Root, 1)...)
	lines = append(lines, renderExternalNodes(g.Edges)...)

	for _, e := range g.Edges {
		lines = append(lines, renderPackageEdge(e))
	}

	lines = append(lines, "@enduml")
	return strings.Join(lines, "\n") + "\n", nil
}

// renderPackageTree renders every one of parent's Children as either a
// nested "package ... { }" block (if the child itself has children) or
// a single-line leaf declaration (if it does not), at the given
// indentation depth. parent itself is never rendered here:
// PackageGraph.Root has no package declaration of its own (see
// RenderPackageGraph's HasRootPackage handling), mirroring how Render
// leaves the root package's own Entries un-wrapped.
func renderPackageTree(parent *diagram.PackageNode, depth int) []string {
	var lines []string
	for _, c := range parent.Children {
		lines = append(lines, renderPackageNode(c, depth)...)
	}
	return lines
}

// renderPackageNode renders node itself: a single-line "package Name
// as alias" declaration if it has no children (there is nothing to
// nest inside a body), or a "package Name as alias { ... }" block
// containing its children otherwise. This mirrors renderClass's own
// "only open a body block when there is something to put in it" rule.
func renderPackageNode(node *diagram.PackageNode, depth int) []string {
	indent := strings.Repeat(indentUnit, depth)
	alias := packageAlias(node.Path)

	if len(node.Children) == 0 {
		return []string{fmt.Sprintf("%spackage %s as %s", indent, node.Name, alias)}
	}

	lines := []string{fmt.Sprintf("%spackage %s as %s {", indent, node.Name, alias)}
	lines = append(lines, renderPackageTree(node, depth+1)...)
	lines = append(lines, indent+"}")
	return lines
}

// renderPackageEdge renders one PackageEdge as an arrow line: a Mutual
// edge uses PlantUML's colored arrow syntax
// "<-[#red,plain,thickness=4]->" so it stands out as a warning; any
// other edge uses a plain "-->".
func renderPackageEdge(e diagram.PackageEdge) string {
	to := packageGraphID(e.To)
	if e.External {
		to = externalNodeID(e.To)
	}

	arrow := "-->"
	if e.Mutual {
		arrow = "<-[#red,plain,thickness=4]->"
	}

	return indentUnit + packageGraphID(e.From) + " " + arrow + " " + to
}

// packageGraphID turns a package directory path (as in
// gocode.Package.Dir, e.g. "product/attribute", or "." for the
// analyzed root) into its package-diagram alias, reusing packageAlias
// for every path except ".", which maps to the literal alias "root" —
// the same special case RenderPackageGraph uses for the root package's
// own declaration line, and the same one
// internal/render/mermaid.packagePathID makes for the equivalent
// Mermaid node ID.
func packageGraphID(p string) string {
	if p == "." || p == "" {
		return "root"
	}
	return packageAlias(p)
}

// renderExternalNodes renders one light-colored "package ... as ...
// #DDDDDD" declaration per distinct external import path referenced
// in edges, sorted for determinism — matching php-class-diagram's own
// convention of drawing an out-of-project dependency in a muted color
// so it reads as background context rather than part of the analyzed
// project.
func renderExternalNodes(edges []diagram.PackageEdge) []string {
	paths := externalImportPaths(edges)
	if len(paths) == 0 {
		return nil
	}

	lines := make([]string, 0, len(paths))
	for _, p := range paths {
		lines = append(lines, fmt.Sprintf(`%spackage "%s" as %s #DDDDDD`, indentUnit, p, externalNodeID(p)))
	}
	return lines
}

// externalImportPaths returns the sorted, deduplicated list of import
// paths that edges references as External targets.
func externalImportPaths(edges []diagram.PackageEdge) []string {
	seen := map[string]bool{}
	var paths []string
	for _, e := range edges {
		if !e.External || seen[e.To] {
			continue
		}
		seen[e.To] = true
		paths = append(paths, e.To)
	}
	sort.Strings(paths)
	return paths
}

// externalNodeID turns a raw import path (e.g. "encoding/json") into
// its PlantUML package alias. The "ext_" prefix guarantees an external
// node's alias never collides with an analyzed package's own alias,
// which is always derived from a directory path and never carries this
// prefix.
func externalNodeID(importPath string) string {
	return "ext_" + packageAlias(importPath)
}
