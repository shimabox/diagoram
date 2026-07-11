package diagram

import "testing"

// TestBuild_Implements exercises Phase 5A's interface implementation
// heuristic end to end (via Build, not buildImplementationEdges
// directly) against the "implements" fixture, which is purpose-built
// to cover every documented rule at once:
//   - Point implements Named directly (its own Name() string method).
//   - Circle implements Named only through Point's Name(), promoted by
//     one level of struct embedding (Circle itself declares no Name()).
//   - Square implements Named directly, independently of Point/Circle.
//   - Sized has no implementer: nothing declares a matching Size() int.
//   - Empty has zero methods and must never appear as an
//     Implementation edge target, even though every struct trivially
//     "matches" a no-method interface.
//   - Labeled's Name() int must not match Point/Circle/Square's
//     Name() string: same method name, different result type.
//   - extras.Widget implements the root package's Describable, proving
//     detection considers every (struct, interface) pair in the whole
//     analyzed set, including across packages that do not import one
//     another.
func TestBuild_Implements(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/implements"))

	var got []Edge
	for _, e := range d.Edges {
		if e.Kind == Implementation {
			got = append(got, e)
		}
	}

	want := []Edge{
		{From: "Circle", To: "Named", Kind: Implementation},
		{From: "Point", To: "Named", Kind: Implementation},
		{From: "Square", To: "Named", Kind: Implementation},
		{From: "extras_Widget", To: "Describable", Kind: Implementation},
	}
	if len(got) != len(want) {
		t.Fatalf("Implementation edges = %+v, want %+v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Implementation edges[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}
