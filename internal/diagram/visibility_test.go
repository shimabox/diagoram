package diagram

import (
	"testing"

	"github.com/shimabox/diagoram/internal/gocode"
)

func TestFilterUnexportedRemovesPrivateMemberEdges(t *testing.T) {
	pkg := &gocode.Package{Dir: ".", Name: "sample"}
	pkg.Structs = []*gocode.Struct{
		{Name: "Public", Fields: []gocode.Field{
			{Name: "detail", Type: gocode.TypeRef{Name: "Detail", String: "Detail"}},
			{Name: "Visible", Type: gocode.TypeRef{Name: "Other", String: "Other"}, Exported: true},
		}},
		{Name: "Detail"},
		{Name: "Other"},
	}

	d := Build([]*gocode.Package{pkg})
	filtered := FilterUnexported(d)
	want := Edge{From: "Public", To: "Other", Kind: Dependency}
	if len(filtered.Edges) != 1 || filtered.Edges[0] != want {
		t.Fatalf("filtered edges = %+v, want %+v", filtered.Edges, []Edge{want})
	}
}

func TestFilterAliasesRemovesAliasEntriesAndEdges(t *testing.T) {
	d := Build(mustParse(t, fixturesDir+"/named-types"))
	filtered := FilterAliases(d)
	if alias := findEntry(filtered.Root, "ItemAlias"); alias != nil {
		t.Fatalf("ItemAlias = %+v, want it removed", alias)
	}
	if item := findEntry(filtered.Root, "Item"); item == nil {
		t.Fatal("Item was removed with its alias")
	}
	for _, edge := range filtered.Edges {
		if edge.From == "ItemAlias" || edge.To == "ItemAlias" {
			t.Errorf("alias edge survived: %+v", edge)
		}
	}
}
