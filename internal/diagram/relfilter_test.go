package diagram

import (
	"sort"
	"strings"
	"testing"
)

// entryNames returns the sorted list of Name for every Entry in the
// tree rooted at node, for compact test assertions.
func entryNames(node *PackageNode) []string {
	var names []string
	var walk func(n *PackageNode)
	walk = func(n *PackageNode) {
		for _, e := range n.Entries {
			names = append(names, e.Name)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(node)
	sort.Strings(names)
	return names
}

// TestFilterByRelTarget exercises the "multi-package" fixture's edge
// shape (product.Product --> attribute.Color <-- config.Config, with
// no direct edge between Product and Config) to pin down depth
// semantics precisely: depth 0 keeps only the targets themselves,
// depth 1 additionally reaches Color (1 hop from Product), and depth 2
// reaches all the way to Config (Product -> Color -> Config is 2
// hops), even though Config was never named as a target.
func TestFilterByRelTarget(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		depth   int
		want    []string
	}{
		{"depth 0 keeps only the target", []string{"Product"}, 0, []string{"Product"}},
		{"depth 1 reaches the directly-connected Color", []string{"Product"}, 1, []string{"Color", "Product"}},
		{"depth 2 reaches Config through Color", []string{"Product"}, 2, []string{"Color", "Config", "Product"}},
		{"negative depth clamps to 0", []string{"Product"}, -5, []string{"Product"}},
		{"pkg-qualified name matches the same Entry as the bare name", []string{"attribute.Color"}, 0, []string{"Color"}},
		{"multiple targets union their reachable sets", []string{"Product", "Config"}, 0, []string{"Config", "Product"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Build(mustParse(t, fixturesDir+"/multi-package"))

			filtered, err := FilterByRelTarget(d, tt.targets, tt.depth)
			if err != nil {
				t.Fatalf("FilterByRelTarget(%v, %d): %v", tt.targets, tt.depth, err)
			}

			got := entryNames(filtered.Root)
			if len(got) != len(tt.want) {
				t.Fatalf("entries = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entries = %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

// TestFilterByRelTarget_PrunesEmptyPackagesAndDeadEdges makes sure
// that, after filtering down to just Product, the config package (now
// entirely empty) is dropped from the tree and no edge dangles to a
// filtered-out Entry.
func TestFilterByRelTarget_PrunesEmptyPackagesAndDeadEdges(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/multi-package"))

	filtered, err := FilterByRelTarget(d, []string{"Product"}, 0)
	if err != nil {
		t.Fatalf("FilterByRelTarget: %v", err)
	}

	if findChild(filtered.Root, "config") != nil {
		t.Errorf("filtered tree still has an (empty) config package: %+v", filtered.Root)
	}
	if len(filtered.Edges) != 0 {
		t.Errorf("Edges = %+v, want none (Color, the only edge target, was filtered out at depth 0)", filtered.Edges)
	}
}

// TestFilterByRelTarget_NotFound makes sure an unresolvable target
// produces a *RelTargetNotFoundError listing every real type name as a
// candidate, rather than silently returning an empty or partial
// Diagram.
func TestFilterByRelTarget_NotFound(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/multi-package"))

	_, err := FilterByRelTarget(d, []string{"NoSuchType"}, 1)
	if err == nil {
		t.Fatal("FilterByRelTarget(NoSuchType): want an error, got nil")
	}
	notFound, ok := err.(*RelTargetNotFoundError)
	if !ok {
		t.Fatalf("FilterByRelTarget(NoSuchType) error type = %T, want *RelTargetNotFoundError", err)
	}
	if len(notFound.Missing) != 1 || notFound.Missing[0] != "NoSuchType" {
		t.Errorf("Missing = %v, want [NoSuchType]", notFound.Missing)
	}
	wantCandidates := []string{"Color", "Config", "Product"}
	if len(notFound.Candidates) != len(wantCandidates) {
		t.Fatalf("Candidates = %v, want %v", notFound.Candidates, wantCandidates)
	}
	for i, c := range wantCandidates {
		if notFound.Candidates[i] != c {
			t.Errorf("Candidates = %v, want %v", notFound.Candidates, wantCandidates)
			break
		}
	}

	msg := err.Error()
	if !strings.Contains(msg, "NoSuchType") || !strings.Contains(msg, "Product") {
		t.Errorf("Error() = %q, want it to mention NoSuchType and a candidate like Product", msg)
	}
}

// TestFilterByRelTarget_NoTargetsIsNoop makes sure an empty targets
// list returns d unchanged rather than an empty Diagram.
func TestFilterByRelTarget_NoTargetsIsNoop(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/multi-package"))

	filtered, err := FilterByRelTarget(d, nil, 1)
	if err != nil {
		t.Fatalf("FilterByRelTarget(nil): %v", err)
	}
	if filtered != d {
		t.Errorf("FilterByRelTarget(nil, ...) = %p, want the same Diagram %p returned unchanged", filtered, d)
	}
}
