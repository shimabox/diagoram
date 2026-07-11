package diagram

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/shimabox/diagoram/internal/gocode"
)

// SummaryOptions controls which counts and relationships Summary
// includes. Its fields mirror render.Options' own display flags, but
// it is a separate type: internal/render (and its mermaid
// implementation) depends on internal/diagram, so diagram cannot
// import render.Options without an import cycle. Callers (cli.Run)
// simply copy the same parsed flag values into both.
type SummaryOptions struct {
	// HideUnexported excludes unexported types and excludes unexported
	// fields/methods from the fields=/methods= counts (--hide-unexported).
	HideUnexported bool
	// DisableFields omits the fields=N count entirely (--disable-fields).
	DisableFields bool
	// DisableMethods omits the methods=N count entirely (--disable-methods).
	DisableMethods bool
	// DisableImplements omits incoming Implementation relationships
	// ("implements: ...") from every interface's line
	// (--disable-implements).
	DisableImplements bool
}

// entrySummaryMeta records the facts about an Entry that Summary needs
// once it has left the tree walk that found it: its owning package's
// directory path (for same-package vs. "pkg.Type" display) and name.
type entrySummaryMeta struct {
	name        string
	dir         string
	packageName string
}

// Summary returns d as a plain-text listing of the analysis:
//
//	diagoram: N packages, M structs, K interfaces
//
//	product/
//	  Product (struct)  fields=3 methods=1  → attribute.Color
//
//	product/attribute/
//	  Color (struct)  fields=2 methods=0
//
// One block is printed per package directory that owns at least one
// Entry (the root package, if it owns any Entries, is headed "." to
// match how the rest of diagoram spells it); "N packages" counts
// exactly those printed blocks. Each Entry's line shows its kind,
// field/method counts (as filtered by opt), its sorted, deduplicated
// outgoing Dependency/Embedding targets (prefixed "→ ", qualified as
// "pkg.Type" when the target lives in a different package directory),
// and — for interfaces only, unless opt.DisableImplements — the sorted
// list of structs that implement it (prefixed "← implements: ").
func Summary(d *Diagram, opt SummaryOptions) string {
	if opt.HideUnexported {
		d = FilterUnexported(d)
	}
	metas := map[string]entrySummaryMeta{}
	collectSummaryMeta(d.Root, metas)

	outgoing := map[string][]Edge{}
	implementedBy := map[string][]string{}
	for _, e := range d.Edges {
		if e.Kind == Implementation {
			if opt.DisableImplements {
				continue
			}
			from, ok := metas[e.From]
			to, ok2 := metas[e.To]
			if !ok || !ok2 {
				continue
			}
			implementedBy[e.To] = append(implementedBy[e.To], qualifiedSummaryName(from, to.dir))
			continue
		}
		outgoing[e.From] = append(outgoing[e.From], e)
	}
	for k := range implementedBy {
		names := implementedBy[k]
		sort.Strings(names)
		implementedBy[k] = names
	}

	var structCount, ifaceCount, namedTypeCount, pkgCount int
	var blocks []string
	var walk func(node *PackageNode)
	walk = func(node *PackageNode) {
		for _, e := range node.Entries {
			switch e.Kind {
			case KindStruct:
				structCount++
			case KindInterface:
				ifaceCount++
			case KindNamedType:
				namedTypeCount++
			}
		}
		if len(node.Entries) > 0 {
			pkgCount++
			blocks = append(blocks, renderSummaryBlock(node, metas, outgoing, implementedBy, opt))
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(d.Root)

	header := fmt.Sprintf("diagoram: %d packages, %d structs, %d interfaces, %d named types\n", pkgCount, structCount, ifaceCount, namedTypeCount)
	if len(blocks) == 0 {
		return header
	}
	return header + "\n" + strings.Join(blocks, "\n\n") + "\n"
}

// collectSummaryMeta records every Entry in the tree rooted at node
// into out, keyed by Entry.ID.
func collectSummaryMeta(node *PackageNode, out map[string]entrySummaryMeta) {
	for _, e := range node.Entries {
		out[e.ID] = entrySummaryMeta{name: e.Name, dir: node.Path, packageName: node.PackageName}
	}
	for _, c := range node.Children {
		collectSummaryMeta(c, out)
	}
}

// qualifiedSummaryName renders m as seen from an Entry declared in
// viewerDir: its bare name when m is declared in the same directory,
// or "pkg.Name" (using the declared package name) otherwise.
func qualifiedSummaryName(m entrySummaryMeta, viewerDir string) string {
	if m.dir == viewerDir {
		return m.name
	}
	if m.packageName != "" {
		return m.packageName + "." + m.name
	}
	if m.dir == "" {
		return "." + m.name
	}
	return lastPathSegment(m.dir) + "." + m.name
}

// renderSummaryBlock renders one package directory's header line
// followed by one aligned line per Entry declared directly in it.
func renderSummaryBlock(node *PackageNode, metas map[string]entrySummaryMeta, outgoing map[string][]Edge, implementedBy map[string][]string, opt SummaryOptions) string {
	title := node.Path
	if title == "" {
		title = "."
	} else {
		title += "/"
	}

	var body strings.Builder
	tw := tabwriter.NewWriter(&body, 0, 0, 1, ' ', 0)
	for _, e := range node.Entries {
		kindLabel := "(struct)"
		if e.Kind == KindInterface {
			kindLabel = "(interface)"
		} else if e.Kind == KindNamedType {
			kindLabel = "(" + NamedTypeLabel(e.NamedType) + ")"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", e.Name, kindLabel, summaryDetails(e, node.Path, metas, outgoing[e.ID], implementedBy[e.ID], opt))
	}
	tw.Flush()

	return title + "\n" + strings.TrimRight(body.String(), "\n")
}

// summaryDetails renders one Entry's trailing columns: its
// fields=/methods= counts (as enabled by opt), its outgoing
// Dependency/Embedding targets, and its incoming Implementation
// edges.
func summaryDetails(e *Entry, ownerDir string, metas map[string]entrySummaryMeta, outEdges []Edge, implementers []string, opt SummaryOptions) string {
	var parts []string

	if e.Kind == KindStruct {
		fields := e.Fields
		methods := e.Methods
		if opt.HideUnexported {
			fields = ExportedFields(fields)
			methods = ExportedMethods(methods)
		}
		var counts []string
		if !opt.DisableFields {
			counts = append(counts, "fields="+strconv.Itoa(len(fields)))
		}
		if !opt.DisableMethods {
			counts = append(counts, "methods="+strconv.Itoa(len(methods)))
		}
		if len(counts) > 0 {
			parts = append(parts, strings.Join(counts, " "))
		}
	} else if e.Kind == KindNamedType {
		if e.NamedType != nil {
			if e.NamedType.Kind == gocode.NamedFunc {
				parts = append(parts, "signature="+e.NamedType.Underlying.String)
			}
			constants := e.NamedType.Constants
			if opt.HideUnexported {
				constants = ExportedConstants(constants)
			}
			if len(constants) > 0 {
				parts = append(parts, "constants="+strconv.Itoa(len(constants)))
			}
		}
		if !opt.DisableMethods {
			methods := e.Methods
			if opt.HideUnexported {
				methods = ExportedMethods(methods)
			}
			parts = append(parts, "methods="+strconv.Itoa(len(methods)))
		}
	} else if e.Kind == KindInterface && !opt.DisableMethods {
		methods := e.Methods
		if opt.HideUnexported {
			methods = ExportedMethods(methods)
		}
		parts = append(parts, "methods="+strconv.Itoa(len(methods)))
	}

	if targets := sortedTargetNames(outEdges, ownerDir, metas); len(targets) > 0 {
		parts = append(parts, "→ "+strings.Join(targets, ", "))
	}
	if len(implementers) > 0 {
		parts = append(parts, "← implements: "+strings.Join(implementers, ", "))
	}
	return strings.Join(parts, "  ")
}

// sortedTargetNames returns the sorted, deduplicated display names of
// edges' targets, qualified as seen from ownerDir (see
// qualifiedSummaryName).
func sortedTargetNames(edges []Edge, ownerDir string, metas map[string]entrySummaryMeta) []string {
	seen := map[string]bool{}
	var names []string
	for _, e := range edges {
		to, ok := metas[e.To]
		if !ok {
			continue
		}
		name := qualifiedSummaryName(to, ownerDir)
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
