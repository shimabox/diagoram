// Package diagram builds the intermediate representation (IR) that
// diagoram's renderers consume: a package tree of Entries (structs and
// interfaces) plus the Edges (dependencies and embeddings) between
// them. It knows about diagramming concepts (packages-as-namespaces,
// edges, safe identifiers) but nothing about any particular output
// format; that is the job of internal/render and its implementations.
package diagram

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/shimabox/diagoram/internal/gocode"
)

// Kind distinguishes the two entry shapes diagoram draws.
type Kind int

const (
	// KindStruct marks an Entry built from a gocode.Struct.
	KindStruct Kind = iota
	// KindInterface marks an Entry built from a gocode.Interface.
	KindInterface
)

// EdgeKind distinguishes the relationships diagoram draws between
// Entries.
type EdgeKind int

const (
	// Dependency is a "uses" relationship: a field type or a method
	// parameter/result type refers to another Entry.
	Dependency EdgeKind = iota
	// Embedding is a Go struct/interface embedding relationship.
	Embedding
)

// Diagram is the fully built intermediate representation: the package
// tree and every edge between the Entries it contains.
type Diagram struct {
	// Root is the tree's synthetic root node. It represents the
	// analyzed directory itself: a gocode.Package with Dir "." (if
	// any) contributes its Entries directly to Root, and every other
	// package contributes to a descendant reached by splitting its Dir
	// on "/".
	Root *PackageNode
	// Edges holds every dependency/embedding edge, sorted
	// deterministically (by From, then Kind, then To) and deduplicated
	// (at most one edge per (From, To, Kind) triple, no self-edges).
	Edges []Edge
}

// PackageNode is one node of the package tree: a single directory
// segment, the Entries declared directly in it, and its child
// directories.
type PackageNode struct {
	// Name is this node's single path segment (e.g. "attribute"). The
	// root node's Name is "".
	Name string
	// Path is the full path from the root, e.g. "product/attribute".
	// The root node's Path is "".
	Path string
	// Children are this node's immediate subdirectories, sorted by
	// Name.
	Children []*PackageNode
	// Entries are the structs/interfaces declared directly in this
	// node's directory, sorted by Name.
	Entries []*Entry
}

// Entry is one struct or interface, ready to be rendered.
type Entry struct {
	// ID is a diagram-wide unique, identifier-safe name (e.g.
	// "product_attribute_Color"). It is derived from the owning
	// package's directory path and the type's name, with any character
	// that is not a letter, digit, or underscore replaced by "_".
	ID string
	// Name is the type's display name, as declared in source.
	Name string
	// Kind reports whether this Entry is a struct or an interface.
	Kind Kind
	// Doc is the first line of the type's doc comment, or "" if none.
	Doc string
	// Fields are the type's fields (always empty for interfaces).
	Fields []gocode.Field
	// Methods are the type's methods (struct methods, or an
	// interface's own method set — embedded interfaces' methods are
	// not flattened in).
	Methods []gocode.Method
}

// Edge is one directed relationship between two Entries, identified by
// their IDs.
type Edge struct {
	// From is the source Entry's ID.
	From string
	// To is the target Entry's ID.
	To string
	// Kind is the relationship type.
	Kind EdgeKind
	// ToCollection reports whether the reference that produced this
	// edge was to a slice/array or a map (used by renderers to hint at
	// multiplicity). When an (From, To, Kind) pair is backed by
	// several type references, ToCollection is true if any of them
	// was a collection.
	ToCollection bool
}

// unsafeIDChar matches any rune that is not safe to use in a Mermaid
// or PlantUML identifier.
var unsafeIDChar = regexp.MustCompile(`[^A-Za-z0-9_]`)

// sanitizeID replaces every character in s that is not a letter,
// digit, or underscore with "_".
func sanitizeID(s string) string {
	return unsafeIDChar.ReplaceAllString(s, "_")
}

// idPrefix turns a gocode.Package.Dir into the ID prefix for entries
// declared in it: "." (the root package) has no prefix; any other
// directory has its "/" separators (and any other unsafe character)
// replaced with "_".
func idPrefix(dir string) string {
	if dir == "." || dir == "" {
		return ""
	}
	return sanitizeID(dir)
}

// entryID builds an Entry.ID from the owning package's directory and
// the type's name.
func entryID(dir, name string) string {
	prefix := idPrefix(dir)
	if prefix == "" {
		return sanitizeID(name)
	}
	return prefix + "_" + sanitizeID(name)
}

// entryKey identifies a declared type by the directory it lives in and
// its name, for resolving TypeRefs to Entries.
type entryKey struct {
	Dir  string
	Name string
}

// Build converts the language model produced by gocode.Parse into a
// Diagram: a package tree of Entries, and the Dependency/Embedding
// edges between them.
//
// Dependency edges come from struct fields and method
// parameters/results; Embedding edges come from struct/interface
// embedding. A TypeRef only becomes an edge when it resolves to an
// Entry that Build itself is building (same package when
// TypeRef.PkgName is "", or another analyzed package resolved via the
// referencing package's imports otherwise): primitives, unresolved
// packages, and generic type parameters are silently excluded. Self-
// edges are never produced, and duplicate (From, To, Kind) edges are
// merged into one.
func Build(pkgs []*gocode.Package) *Diagram {
	root := &PackageNode{}
	nodes := map[string]*PackageNode{".": root, "": root}

	registry := map[entryKey]*Entry{}
	pkgByDir := map[string]*gocode.Package{}
	var dirs []string

	// First pass: build the tree and every Entry, so that edge
	// resolution (second pass) can look any of them up regardless of
	// declaration order across packages.
	for _, pkg := range pkgs {
		pkgByDir[pkg.Dir] = pkg
		dirs = append(dirs, pkg.Dir)
		node := ensureNode(nodes, root, pkg.Dir)

		for _, s := range pkg.Structs {
			e := &Entry{
				ID:      entryID(pkg.Dir, s.Name),
				Name:    s.Name,
				Kind:    KindStruct,
				Doc:     s.Doc,
				Fields:  s.Fields,
				Methods: s.Methods,
			}
			node.Entries = append(node.Entries, e)
			registry[entryKey{pkg.Dir, s.Name}] = e
		}
		for _, i := range pkg.Interfaces {
			e := &Entry{
				ID:      entryID(pkg.Dir, i.Name),
				Name:    i.Name,
				Kind:    KindInterface,
				Doc:     i.Doc,
				Methods: i.Methods,
			}
			node.Entries = append(node.Entries, e)
			registry[entryKey{pkg.Dir, i.Name}] = e
		}
	}

	sortTree(root)

	// Second pass: resolve edges now that every Entry is known.
	edges := map[edgeKey]*Edge{}
	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			self := registry[entryKey{pkg.Dir, s.Name}]
			for _, ref := range s.Embeds {
				addEdge(edges, registry, pkgByDir, dirs, pkg.Dir, self.ID, ref, Embedding)
			}
			for _, f := range s.Fields {
				addEdge(edges, registry, pkgByDir, dirs, pkg.Dir, self.ID, f.Type, Dependency)
			}
			for _, m := range s.Methods {
				addMethodEdges(edges, registry, pkgByDir, dirs, pkg.Dir, self.ID, m)
			}
		}
		for _, i := range pkg.Interfaces {
			self := registry[entryKey{pkg.Dir, i.Name}]
			for _, ref := range i.Embeds {
				addEdge(edges, registry, pkgByDir, dirs, pkg.Dir, self.ID, ref, Embedding)
			}
			for _, m := range i.Methods {
				addMethodEdges(edges, registry, pkgByDir, dirs, pkg.Dir, self.ID, m)
			}
		}
	}

	sortedEdges := make([]Edge, 0, len(edges))
	for _, e := range edges {
		sortedEdges = append(sortedEdges, *e)
	}
	sort.Slice(sortedEdges, func(i, j int) bool {
		a, b := sortedEdges[i], sortedEdges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.To < b.To
	})

	return &Diagram{Root: root, Edges: sortedEdges}
}

// edgeKey identifies an (From, To, Kind) triple for deduplication.
type edgeKey struct {
	From string
	To   string
	Kind EdgeKind
}

// addMethodEdges adds Dependency edges for every parameter and result
// TypeRef of m.
func addMethodEdges(edges map[edgeKey]*Edge, registry map[entryKey]*Entry, pkgByDir map[string]*gocode.Package, dirs []string, fromDir, fromID string, m gocode.Method) {
	for _, ref := range m.Params {
		addEdge(edges, registry, pkgByDir, dirs, fromDir, fromID, ref, Dependency)
	}
	for _, ref := range m.Results {
		addEdge(edges, registry, pkgByDir, dirs, fromDir, fromID, ref, Dependency)
	}
}

// addEdge resolves ref against the package rooted at fromDir and, if
// it names another known Entry (and is not a self-reference), records
// or merges an edge of the given kind from fromID to it.
func addEdge(edges map[edgeKey]*Edge, registry map[entryKey]*Entry, pkgByDir map[string]*gocode.Package, dirs []string, fromDir, fromID string, ref gocode.TypeRef, kind EdgeKind) {
	target, ok := resolveTypeRef(registry, pkgByDir, dirs, fromDir, ref)
	if !ok || target.ID == fromID {
		return
	}

	key := edgeKey{From: fromID, To: target.ID, Kind: kind}
	if existing, ok := edges[key]; ok {
		if ref.IsSlice || ref.IsMap {
			existing.ToCollection = true
		}
		return
	}
	edges[key] = &Edge{
		From:         fromID,
		To:           target.ID,
		Kind:         kind,
		ToCollection: ref.IsSlice || ref.IsMap,
	}
}

// resolveTypeRef finds the Entry that ref refers to, if any. When
// ref.PkgName is "" the lookup happens in fromDir's own package;
// otherwise ref.PkgName is resolved to an import of fromDir's package
// (matching an explicit alias, or the import path's last segment as
// the default local name), and that import's path is matched against
// every known package directory's path suffix to find the target
// directory.
func resolveTypeRef(registry map[entryKey]*Entry, pkgByDir map[string]*gocode.Package, dirs []string, fromDir string, ref gocode.TypeRef) (*Entry, bool) {
	if ref.Name == "" {
		return nil, false
	}

	targetDir := fromDir
	if ref.PkgName != "" {
		pkg := pkgByDir[fromDir]
		if pkg == nil {
			return nil, false
		}
		imp, ok := findImport(pkg.Imports, ref.PkgName)
		if !ok {
			return nil, false
		}
		targetDir, ok = resolveImportDir(dirs, imp.Path)
		if !ok {
			return nil, false
		}
	}

	e, ok := registry[entryKey{targetDir, ref.Name}]
	return e, ok
}

// findImport finds the import declaration that the identifier
// qualifier refers to: an import whose explicit Alias equals
// qualifier, or (failing that) an unaliased import whose path's last
// segment equals qualifier (the default local name Go gives an
// import).
func findImport(imports []gocode.Import, qualifier string) (gocode.Import, bool) {
	for _, imp := range imports {
		if imp.Alias != "" && imp.Alias == qualifier {
			return imp, true
		}
	}
	for _, imp := range imports {
		if imp.Alias == "" && path.Base(imp.Path) == qualifier {
			return imp, true
		}
	}
	return gocode.Import{}, false
}

// resolveImportDir matches importPath against every known package
// directory dirs, returning the one whose slash-form path is the
// longest suffix of importPath (matched on a "/" boundary). This is a
// heuristic: without go/packages or a module cache, the only fact
// available is that an analyzed package's own subdirectory structure
// is mirrored at the end of any import path pointing into it.
func resolveImportDir(dirs []string, importPath string) (string, bool) {
	best := ""
	bestLen := -1
	for _, d := range dirs {
		if d == "." {
			continue
		}
		if importPath == d || strings.HasSuffix(importPath, "/"+d) {
			if len(d) > bestLen {
				best, bestLen = d, len(d)
			}
		}
	}
	if bestLen < 0 {
		return "", false
	}
	return best, true
}

// ensureNode returns the PackageNode for dir, creating it and any
// missing ancestor nodes (linking each into its parent's Children) as
// needed.
func ensureNode(nodes map[string]*PackageNode, root *PackageNode, dir string) *PackageNode {
	if dir == "." || dir == "" {
		return root
	}
	if n, ok := nodes[dir]; ok {
		return n
	}

	parentDir := path.Dir(dir)
	parent := ensureNode(nodes, root, parentDir)

	node := &PackageNode{Name: path.Base(dir), Path: dir}
	parent.Children = append(parent.Children, node)
	nodes[dir] = node
	return node
}

// sortTree recursively sorts node's Entries by Name and Children by
// Name, so that Build's output is deterministic regardless of the
// input packages' order.
func sortTree(node *PackageNode) {
	sort.Slice(node.Entries, func(i, j int) bool {
		return node.Entries[i].Name < node.Entries[j].Name
	})
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, c := range node.Children {
		sortTree(c)
	}
}
