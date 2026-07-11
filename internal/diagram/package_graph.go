package diagram

import (
	"sort"
	"strings"

	"github.com/shimabox/diagoram/internal/gocode"
)

// PackageEdge is one directed dependency between two packages, derived
// from an import declaration.
type PackageEdge struct {
	// From is the importing package's directory path, exactly as in
	// gocode.Package.Dir (e.g. "product" or "." for the analyzed root
	// package).
	From string
	// To is the imported package's directory path when it resolves to
	// one of the analyzed packages, or its raw import path (e.g.
	// "fmt", "encoding/json") when External is true.
	To string
	// Mutual reports whether To also directly imports From: a direct,
	// two-package import cycle. Only that direct case is detected —
	// indirect cycles spanning three or more packages (a general
	// strongly connected component analysis) are out of scope for now
	// (see BuildPackageGraph's doc comment). A Mutual edge represents
	// both directions at once: the reverse (To, From) pair never
	// appears as its own PackageEdge.
	Mutual bool
	// External reports whether To lies outside the analyzed package
	// set (the standard library, or another module). External edges
	// are only ever produced when BuildPackageGraph is called with
	// showExternal = true; they are never Mutual, since an external
	// package's own imports are not analyzed.
	External bool
}

// PackageGraph is the intermediate representation for a package
// dependency diagram: the analyzed packages' directory tree (built the
// same way as Diagram.Root, so renderers can nest packages
// consistently) and the import edges between them.
type PackageGraph struct {
	// Root is the tree of analyzed packages. Unlike Diagram.Root,
	// PackageNode.Entries is always empty here: a package graph draws
	// packages, not the types inside them.
	Root *PackageNode
	// Edges holds every package dependency edge, sorted deterministically
	// (by From, then To) and deduplicated: at most one edge per
	// unordered pair of analyzed packages (merged into a single Mutual
	// edge when both directions are present), plus at most one edge per
	// (From, external import path) pair.
	Edges []PackageEdge
	// HasRootPackage reports whether the analyzed root directory (Dir
	// ".") is itself a Go package, as opposed to merely being an
	// ancestor directory of other analyzed packages. Renderers use this
	// to decide whether to draw a node for the root package itself: it
	// is never wrapped in a subgraph, mirroring how Diagram renders the
	// root package's own Entries un-namespaced.
	HasRootPackage bool
}

// pkgPair identifies an unordered relationship between two package
// paths, used to detect and dedupe mutual (bidirectional) edges.
type pkgPair struct {
	From, To string
}

// BuildPackageGraph aggregates the import relationships between pkgs
// into a package dependency graph: one PackageEdge per pair of
// analyzed packages where one imports the other, with direct
// two-package import cycles (A imports B and B imports A) flagged via
// Mutual.
//
// Only imports that resolve to another package in pkgs become
// (non-external) edges. modulePath, if non-empty (see
// ReadModulePath), is used to resolve import paths to package
// directories exactly: importPath must equal modulePath or have it as
// a "/"-bounded prefix, and the remaining suffix must exactly name one
// of pkgs' directories. When modulePath is "" (no go.mod was found),
// resolution falls back to the same longest-suffix heuristic
// diagram.Build's own TypeRef resolution uses (resolveImportDir).
//
// When showExternal is true, imports that do not resolve to any
// analyzed package are additionally recorded as External edges, keyed
// by their raw import path rather than a directory; when showExternal
// is false they are silently dropped, matching php-class-diagram's
// convention of hiding packages outside the analyzed project by
// default.
//
// Detecting indirect import cycles across three or more packages (a
// general strongly connected component analysis) is left as a future
// enhancement — see the Phase 4 plan's "スコープ外" section. Only
// direct, two-package cycles are flagged here.
func BuildPackageGraph(pkgs []*gocode.Package, modulePath string, showExternal bool) *PackageGraph {
	root := &PackageNode{}
	nodes := map[string]*PackageNode{".": root, "": root}

	var dirs []string
	dirSet := map[string]bool{}
	hasRoot := false
	for _, pkg := range pkgs {
		dirs = append(dirs, pkg.Dir)
		dirSet[pkg.Dir] = true
		if pkg.Dir == "." || pkg.Dir == "" {
			hasRoot = true
			continue
		}
		ensureNode(nodes, root, pkg.Dir)
	}
	sortTree(root)

	directed := map[pkgPair]bool{}
	external := map[pkgPair]bool{}
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			targetDir, ok := resolvePackageImportDir(dirs, dirSet, modulePath, imp.Path)
			if ok {
				if targetDir == pkg.Dir {
					continue // self-import; shouldn't normally happen, but never an edge.
				}
				directed[pkgPair{pkg.Dir, targetDir}] = true
				continue
			}
			if showExternal {
				external[pkgPair{pkg.Dir, imp.Path}] = true
			}
		}
	}

	edges := mergeMutualEdges(directed)
	for pair := range external {
		edges = append(edges, PackageEdge{From: pair.From, To: pair.To, External: true})
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return &PackageGraph{Root: root, Edges: edges, HasRootPackage: hasRoot}
}

// mergeMutualEdges turns a set of directed (From, To) pairs into
// PackageEdges: whenever both (A, B) and (B, A) are present, they
// collapse into a single Mutual edge (canonically ordered so the same
// unordered pair always produces the same From/To regardless of
// iteration order), and every other pair becomes an ordinary directed
// edge.
func mergeMutualEdges(directed map[pkgPair]bool) []PackageEdge {
	var edges []PackageEdge
	done := map[pkgPair]bool{}
	for pair := range directed {
		if done[pair] {
			continue
		}
		done[pair] = true

		reverse := pkgPair{pair.To, pair.From}
		if directed[reverse] {
			done[reverse] = true
			from, to := pair.From, pair.To
			if to < from {
				from, to = to, from
			}
			edges = append(edges, PackageEdge{From: from, To: to, Mutual: true})
			continue
		}
		edges = append(edges, PackageEdge{From: pair.From, To: pair.To})
	}
	return edges
}

// resolvePackageImportDir maps importPath to one of dirs — the
// directory of the analyzed package it refers to — or reports
// ok=false when importPath does not refer to any analyzed package
// (the standard library, another module, or a subdirectory that was
// filtered out of analysis).
//
// When modulePath is known, resolution is exact: importPath must
// equal modulePath or have it as a "/"-bounded prefix, and the
// remaining suffix must be exactly one of dirSet. When modulePath is
// "" (no go.mod was found), resolution falls back to
// resolveImportDir's longest-suffix heuristic.
func resolvePackageImportDir(dirs []string, dirSet map[string]bool, modulePath, importPath string) (string, bool) {
	if modulePath == "" {
		return resolveImportDir(dirs, importPath)
	}

	rest, ok := trimModulePrefix(modulePath, importPath)
	if !ok {
		return "", false
	}
	if rest == "" {
		rest = "."
	}
	return rest, dirSet[rest]
}

// trimModulePrefix reports whether importPath lies within modulePath
// (equal to it, or "/"-prefixed by it) and, if so, returns the
// remaining path suffix ("" for modulePath itself).
func trimModulePrefix(modulePath, importPath string) (string, bool) {
	if importPath == modulePath {
		return "", true
	}
	if rest, ok := strings.CutPrefix(importPath, modulePath+"/"); ok {
		return rest, true
	}
	return "", false
}
