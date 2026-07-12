package diagram

import (
	"fmt"
	"sort"
	"strings"
)

// RelTargetNotFoundError is returned by FilterByRelTarget when one or
// more --rel-target values do not match any Entry in the Diagram being
// filtered. It lists every known type name as a candidate, so a caller
// can print a developer-friendly "did you mean one of these?" error.
type RelTargetNotFoundError struct {
	// Missing holds every --rel-target value that matched no Entry, in
	// the order they were given.
	Missing []string
	// Candidates holds every analyzed type's bare name, sorted and
	// deduplicated.
	Candidates []string
}

// Error implements the error interface.
func (e *RelTargetNotFoundError) Error() string {
	return fmt.Sprintf("no type named %s found. Available types: %s", quoteJoin(e.Missing), strings.Join(e.Candidates, ", "))
}

// quoteJoin renders names as a comma-separated, double-quoted list
// (e.g. `"Foo", "Bar"`) for use in error messages.
func quoteJoin(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(quoted, ", ")
}

// FilterByRelTarget returns a new Diagram (d itself is left untouched)
// containing only the Entries reachable from targets — the --rel-target
// start set — by following d.Edges in either direction up to depth
// hops, plus every Edge whose From and To both survive that filter.
// PackageNodes that end up with no surviving Entry and no surviving
// (non-empty) child are dropped from the result's tree.
//
// Each target in targets is matched against every Entry's bare type
// name (e.g. "Product") and, for Entries outside the root package,
// against a "pkg.Type" qualified name built from the owning package
// directory's last path segment (e.g. "attribute.Color" for the Entry
// declared in package directory "product/attribute") — mirroring how
// the type is referred to from other packages in Go source. A bare
// name that happens to be declared in more than one analyzed package is
// not an error: it simply expands to every Entry sharing that name, all
// of which become start points.
//
// depth < 0 is treated as 0 (only the targets themselves, no
// traversal); the default the CLI applies when --rel-target-depth is
// not given is 1.
//
// If any target matches no Entry at all, FilterByRelTarget returns a
// *RelTargetNotFoundError instead of a Diagram, listing every target
// that could not be resolved and every known type name as a candidate.
// An empty targets list is a no-op: d is returned as-is.
func FilterByRelTarget(d *Diagram, targets []string, depth int) (*Diagram, error) {
	if depth < 0 {
		depth = 0
	}

	index, candidates := buildRelTargetIndex(d.Root)

	start := map[string]bool{}
	var missing []string
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		ids, ok := index[t]
		if !ok {
			missing = append(missing, t)
			continue
		}
		for _, id := range ids {
			start[id] = true
		}
	}
	if len(missing) > 0 {
		return nil, &RelTargetNotFoundError{Missing: missing, Candidates: candidates}
	}
	if len(start) == 0 {
		return d, nil
	}

	keep := bfsReachable(d.Edges, start, depth)
	return rebuildFiltered(d, keep), nil
}

// buildRelTargetIndex walks root and returns a lookup from every name
// a --rel-target value may spell (a bare type name, and — for Entries
// outside the root package — a "pkg.Type" qualified name) to the
// matching Entry IDs, plus the sorted, deduplicated list of every bare
// type name (for RelTargetNotFoundError.Candidates).
func buildRelTargetIndex(root *PackageNode) (map[string][]string, []string) {
	index := map[string][]string{}
	seenNames := map[string]bool{}
	var candidates []string

	var walk func(node *PackageNode)
	walk = func(node *PackageNode) {
		for _, e := range node.Entries {
			if e.Kind == KindPackageFunctions {
				continue
			}
			index[e.Name] = append(index[e.Name], e.ID)
			if node.PackageName != "" {
				qualified := node.PackageName + "." + e.Name
				index[qualified] = append(index[qualified], e.ID)
			} else if node.Path != "" {
				qualified := lastPathSegment(node.Path) + "." + e.Name
				index[qualified] = append(index[qualified], e.ID)
			}
			if !seenNames[e.Name] {
				seenNames[e.Name] = true
				candidates = append(candidates, e.Name)
			}
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(root)

	sort.Strings(candidates)
	return index, candidates
}

// lastPathSegment returns the final "/"-separated segment of p (p
// itself, if it has none).
func lastPathSegment(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// bfsReachable returns the set of Entry IDs reachable from start by
// following edges — treated as undirected, per the Phase 5 plan's
// "from/to 両方向に" — up to depth hops (0 returns start unchanged).
func bfsReachable(edges []Edge, start map[string]bool, depth int) map[string]bool {
	adj := map[string][]string{}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}

	visited := make(map[string]bool, len(start))
	frontier := make([]string, 0, len(start))
	for id := range start {
		visited[id] = true
		frontier = append(frontier, id)
	}

	for hop := 0; hop < depth; hop++ {
		var next []string
		for _, id := range frontier {
			for _, n := range adj[id] {
				if !visited[n] {
					visited[n] = true
					next = append(next, n)
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	return visited
}

// rebuildFiltered returns a new Diagram whose tree only contains the
// Entries in keep and whose Edges only contains edges with both
// endpoints in keep. Entries themselves are reused (not copied): they
// are treated as immutable once Build produces them.
func rebuildFiltered(d *Diagram, keep map[string]bool) *Diagram {
	root := filterNode(d.Root, keep)
	if root == nil {
		root = &PackageNode{}
	}

	var edges []Edge
	reasons := map[edgeKey]EdgeReasons{}
	for _, e := range d.Edges {
		if keep[e.From] && keep[e.To] {
			edges = append(edges, e)
			key := edgeKey{From: e.From, To: e.To, Kind: e.Kind}
			reasons[key] = d.ReasonsFor(e)
		}
	}
	return &Diagram{Root: root, Edges: edges, edgeReasons: reasons}
}

// filterNode returns a copy of node retaining only its Entries in keep
// and its Children after the same filtering (recursively), or nil if
// nothing of node would survive (no kept Entry and no surviving
// Child) — callers filtering the tree's true root must handle a nil
// result themselves (see rebuildFiltered), since the root is never
// meant to disappear outright.
func filterNode(node *PackageNode, keep map[string]bool) *PackageNode {
	var entries []*Entry
	for _, e := range node.Entries {
		if keep[e.ID] {
			entries = append(entries, e)
		}
	}
	var children []*PackageNode
	for _, c := range node.Children {
		if fc := filterNode(c, keep); fc != nil {
			children = append(children, fc)
		}
	}
	if len(entries) == 0 && len(children) == 0 {
		return nil
	}
	return &PackageNode{Name: node.Name, Path: node.Path, PackageName: node.PackageName, Entries: entries, Children: children}
}
