package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/render"
)

// RenderPackageGraph returns g as Mermaid flowchart text
// (https://mermaid.js.org/syntax/flowchart.html): a "flowchart TD"
// header, one node per analyzed package nested into subgraphs that
// mirror the package directory hierarchy, every PackageEdge as an
// arrow line, and finally a "linkStyle" line for each Mutual edge so
// it is drawn as a red, bold, bidirectional arrow. RenderPackageGraph
// never returns a non-nil error; it is declared to return one to
// match Render's shape and to leave room for future validation.
func (r *Renderer) RenderPackageGraph(g *diagram.PackageGraph, _ render.Options) (string, error) {
	lines := []string{"flowchart TD"}

	if g.HasRootPackage {
		lines = append(lines, indentUnit+`root["."]`)
	}
	lines = append(lines, renderPackageTree(g.Root, 1)...)
	lines = append(lines, renderExternalNodes(g.Edges)...)

	var linkStyles []string
	for i, e := range g.Edges {
		lines = append(lines, renderPackageEdge(e))
		if e.Mutual {
			linkStyles = append(linkStyles, fmt.Sprintf("%slinkStyle %d stroke:red,stroke-width:4px", indentUnit, i))
		}
	}
	lines = append(lines, linkStyles...)

	return strings.Join(lines, "\n") + "\n", nil
}

// renderPackageTree renders every one of parent's Children as either a
// nested subgraph (if the child itself has children) or a leaf node
// (if it does not), at the given indentation depth. parent itself is
// never rendered here: PackageGraph.Root has no subgraph of its own
// (see RenderPackageGraph's HasRootPackage handling), mirroring how
// Render leaves the root package's own Entries un-namespaced.
func renderPackageTree(parent *diagram.PackageNode, depth int) []string {
	var lines []string
	for _, c := range parent.Children {
		lines = append(lines, renderPackageNode(c, depth)...)
	}
	return lines
}

// renderPackageNode renders node itself: a leaf `id["Name"]` line if
// it has no children, or a `subgraph id["Name"] ... end` block
// containing its children otherwise.
//
// A package that both has children and participates in an edge (e.g.
// "gamma" importing something while also containing "gamma/sub") is
// never declared twice and never collides: Mermaid treats a
// subgraph's own ID as directly usable as an edge endpoint (connecting
// to the subgraph as a whole), so the subgraph declaration doubles as
// that package's node — there is no separate leaf node to conflict
// with it.
func renderPackageNode(node *diagram.PackageNode, depth int) []string {
	indent := strings.Repeat(indentUnit, depth)
	id := packagePathID(node.Path)

	if len(node.Children) == 0 {
		return []string{fmt.Sprintf(`%s%s["%s"]`, indent, id, node.Name)}
	}

	lines := []string{fmt.Sprintf(`%ssubgraph %s["%s"]`, indent, id, node.Name)}
	lines = append(lines, renderPackageTree(node, depth+1)...)
	lines = append(lines, indent+"end")
	return lines
}

// renderPackageEdge renders one PackageEdge as an arrow line: a Mutual
// edge uses a single bidirectional "<-->" arrow (its "linkStyle" line,
// if any, is emitted separately by RenderPackageGraph so its index can
// track the edge's position among every rendered edge); any other
// edge uses "-->".
func renderPackageEdge(e diagram.PackageEdge) string {
	arrow := "-->"
	if e.Mutual {
		arrow = "<-->"
	}

	to := packagePathID(e.To)
	if e.External {
		to = externalNodeID(e.To)
	}

	return indentUnit + packagePathID(e.From) + " " + arrow + " " + to
}

// renderExternalNodes renders one classDef line (only if edges
// contains at least one External edge) followed by one light-styled
// node declaration per distinct external import path referenced,
// sorted for determinism.
func renderExternalNodes(edges []diagram.PackageEdge) []string {
	paths := externalImportPaths(edges)
	if len(paths) == 0 {
		return nil
	}

	lines := []string{indentUnit + "classDef external fill:#eee,stroke:#bbb,color:#999"}
	for _, p := range paths {
		lines = append(lines, fmt.Sprintf(`%s%s["%s"]:::external`, indentUnit, externalNodeID(p), p))
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

// packagePathID turns a package directory path (as in
// gocode.Package.Dir, e.g. "product/attribute", or "." for the
// analyzed root) into its flowchart node/subgraph ID, reusing the same
// flattening convention as namespaceName.
func packagePathID(p string) string {
	if p == "." || p == "" {
		return "root"
	}
	return namespaceName(p)
}

// externalNodeID turns a raw import path (e.g. "encoding/json") into
// its flowchart node ID. The "ext_" prefix guarantees an external
// node's ID never collides with an analyzed package's own ID, which is
// always derived from a directory path and never carries this prefix.
func externalNodeID(importPath string) string {
	return "ext_" + namespaceName(importPath)
}
