package diagram

import (
	"testing"

	"github.com/shimabox/diagoram/internal/gocode"
)

func mustParsePkgs(t *testing.T, dir string) []*gocode.Package {
	t.Helper()
	pkgs, warnings, err := gocode.Parse(dir, gocode.ParseOptions{})
	if err != nil {
		t.Fatalf("gocode.Parse(%q): %v", dir, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("gocode.Parse(%q) warnings = %+v, want none", dir, warnings)
	}
	return pkgs
}

// findEdge returns the edge from -> to in edges, or nil if there is
// none.
func findEdge(edges []PackageEdge, from, to string) *PackageEdge {
	for i := range edges {
		if edges[i].From == from && edges[i].To == to {
			return &edges[i]
		}
	}
	return nil
}

// TestBuildPackageGraph_DependencyLoops exercises the
// "dependency-loops" fixture end to end: alpha and beta import each
// other directly (Mutual), alpha->gamma is an ordinary one-way
// dependency, gamma->gamma/sub exercises a nested package edge, and
// alpha's "fmt" import is external (excluded unless showExternal).
func TestBuildPackageGraph_DependencyLoops(t *testing.T) {
	dir := fixturesDir + "/dependency-loops"
	pkgs := mustParsePkgs(t, dir)

	modulePath, err := ReadModulePath(dir)
	if err != nil {
		t.Fatalf("ReadModulePath(%q): %v", dir, err)
	}
	if modulePath != "example.com/looptest" {
		t.Fatalf("ReadModulePath(%q) = %q, want %q", dir, modulePath, "example.com/looptest")
	}

	t.Run("default excludes external", func(t *testing.T) {
		g := BuildPackageGraph(pkgs, modulePath, false)

		if len(g.Edges) != 3 {
			t.Fatalf("Edges = %+v, want 3 edges", g.Edges)
		}

		mutual := findEdge(g.Edges, "alpha", "beta")
		if mutual == nil {
			t.Fatalf("no alpha->beta edge in %+v", g.Edges)
		}
		if !mutual.Mutual {
			t.Errorf("alpha->beta.Mutual = false, want true")
		}
		// The reverse direction must not also appear as a separate
		// edge: a mutual pair collapses into one edge.
		if e := findEdge(g.Edges, "beta", "alpha"); e != nil {
			t.Errorf("beta->alpha also present (%+v); mutual pairs must collapse into one edge", e)
		}

		gammaEdge := findEdge(g.Edges, "alpha", "gamma")
		if gammaEdge == nil || gammaEdge.Mutual || gammaEdge.External {
			t.Errorf("alpha->gamma = %+v, want a plain, non-mutual, non-external edge", gammaEdge)
		}

		subEdge := findEdge(g.Edges, "gamma", "gamma/sub")
		if subEdge == nil || subEdge.Mutual || subEdge.External {
			t.Errorf("gamma->gamma/sub = %+v, want a plain, non-mutual, non-external edge", subEdge)
		}

		for _, e := range g.Edges {
			if e.External {
				t.Errorf("edge %+v is External, want no external edges when showExternal=false", e)
			}
		}

		if g.HasRootPackage {
			t.Errorf("HasRootPackage = true, want false (no .go files directly in the fixture root)")
		}
	})

	t.Run("showExternal includes fmt", func(t *testing.T) {
		g := BuildPackageGraph(pkgs, modulePath, true)

		if len(g.Edges) != 4 {
			t.Fatalf("Edges = %+v, want 4 edges", g.Edges)
		}

		fmtEdge := findEdge(g.Edges, "alpha", "fmt")
		if fmtEdge == nil {
			t.Fatalf("no alpha->fmt edge in %+v", g.Edges)
		}
		if !fmtEdge.External {
			t.Errorf("alpha->fmt.External = false, want true")
		}
		if fmtEdge.Mutual {
			t.Errorf("alpha->fmt.Mutual = true, want false (external edges are never mutual)")
		}
	})

	t.Run("tree nests gamma/sub under gamma", func(t *testing.T) {
		g := BuildPackageGraph(pkgs, modulePath, false)

		gamma := findChild(g.Root, "gamma")
		if gamma == nil {
			t.Fatalf("Root has no child %q; children = %+v", "gamma", g.Root.Children)
		}
		if findChild(gamma, "sub") == nil {
			t.Errorf("gamma has no child %q; children = %+v", "sub", gamma.Children)
		}
		if findChild(g.Root, "alpha") == nil || findChild(g.Root, "beta") == nil {
			t.Errorf("Root.Children = %+v, want alpha and beta as direct children", g.Root.Children)
		}
	})
}

// TestBuildPackageGraph_NoModulePath exercises the heuristic
// suffix-matching fallback (resolveImportDir) used when modulePath is
// "" (no go.mod, or ReadModulePath found none), reusing the
// "multi-package" fixture whose import paths happen to match this
// repository's own module path even though we deliberately don't pass
// it here.
func TestBuildPackageGraph_NoModulePath(t *testing.T) {
	dir := fixturesDir + "/multi-package"
	pkgs := mustParsePkgs(t, dir)

	g := BuildPackageGraph(pkgs, "", false)

	if e := findEdge(g.Edges, "product", "product/attribute"); e == nil {
		t.Errorf("Edges = %+v, want product->product/attribute (resolved via suffix heuristic)", g.Edges)
	}
	if e := findEdge(g.Edges, "config", "product/attribute"); e == nil {
		t.Errorf("Edges = %+v, want config->product/attribute (resolved via suffix heuristic)", g.Edges)
	}
}

// TestBuildPackageGraph_Empty makes sure an empty package list
// produces an empty, non-nil graph rather than panicking.
func TestBuildPackageGraph_Empty(t *testing.T) {
	g := BuildPackageGraph(nil, "", false)
	if g == nil || g.Root == nil {
		t.Fatalf("BuildPackageGraph(nil, ...) = %+v, want a non-nil graph with a non-nil Root", g)
	}
	if len(g.Edges) != 0 {
		t.Errorf("Edges = %+v, want none", g.Edges)
	}
	if g.HasRootPackage {
		t.Errorf("HasRootPackage = true, want false")
	}
}

// TestBuildPackageGraph_RootPackage covers a package graph where the
// analyzed root directory (Dir ".") is itself an analyzed package
// that depends on a subpackage, exercising HasRootPackage and the
// "root" node ID.
func TestBuildPackageGraph_RootPackage(t *testing.T) {
	pkgs := []*gocode.Package{
		{Dir: ".", Name: "main", Imports: []gocode.Import{{Path: "example.com/root/child"}}},
		{Dir: "child", Name: "child"},
	}

	g := BuildPackageGraph(pkgs, "example.com/root", false)

	if !g.HasRootPackage {
		t.Errorf("HasRootPackage = false, want true")
	}
	if e := findEdge(g.Edges, ".", "child"); e == nil {
		t.Errorf("Edges = %+v, want .->child", g.Edges)
	}
}
