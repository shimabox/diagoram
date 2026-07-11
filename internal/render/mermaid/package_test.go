package mermaid

import (
	"strings"
	"testing"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/testutil"
)

// buildPackageGraph parses dir, reads its go.mod (if any), and builds
// the resulting PackageGraph.
func buildPackageGraph(t *testing.T, dir string, showExternal bool) *diagram.PackageGraph {
	t.Helper()
	pkgs, warnings, err := gocode.Parse(dir, gocode.ParseOptions{})
	if err != nil {
		t.Fatalf("gocode.Parse(%q): %v", dir, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("gocode.Parse(%q) warnings = %+v, want none", dir, warnings)
	}
	modulePath, err := diagram.ReadModulePath(dir)
	if err != nil {
		t.Fatalf("diagram.ReadModulePath(%q): %v", dir, err)
	}
	return diagram.BuildPackageGraph(pkgs, modulePath, showExternal)
}

// TestRenderPackageGraph_GoldenFixture exercises the "dependency-loops"
// fixture, whose golden output is the reference the Phase 4 plan's
// design was built against: a direct import cycle rendered as one red,
// bold, bidirectional arrow, plus a nested subgraph and an ordinary
// one-way edge.
func TestRenderPackageGraph_GoldenFixture(t *testing.T) {
	dir := fixturesDir + "/dependency-loops"

	t.Run("default hides external packages", func(t *testing.T) {
		g := buildPackageGraph(t, dir, false)

		got, err := New().RenderPackageGraph(g, render.Options{})
		if err != nil {
			t.Fatalf("RenderPackageGraph: %v", err)
		}

		testutil.Golden(t, dir+"/expected-package.mmd", got)
	})

	t.Run("show-external includes fmt", func(t *testing.T) {
		g := buildPackageGraph(t, dir, true)

		got, err := New().RenderPackageGraph(g, render.Options{})
		if err != nil {
			t.Fatalf("RenderPackageGraph: %v", err)
		}

		testutil.Golden(t, dir+"/expected-package-external.mmd", got)
	})
}

// TestRenderPackageGraph_Empty makes sure an empty PackageGraph
// renders to a minimal, valid flowchart with no trailing garbage.
func TestRenderPackageGraph_Empty(t *testing.T) {
	got, err := New().RenderPackageGraph(diagram.BuildPackageGraph(nil, "", false), render.Options{})
	if err != nil {
		t.Fatalf("RenderPackageGraph: %v", err)
	}
	want := "flowchart TD\n"
	if got != want {
		t.Errorf("RenderPackageGraph(empty) = %q, want %q", got, want)
	}
}

// TestRenderPackageGraph_LinkStyleIndexIsStable makes sure the
// linkStyle index always refers to the Mutual edge's actual position
// among the rendered arrow lines, not merely "the first edge" or some
// other coincidence — regressions here would silently misapply the
// red/bold styling to the wrong arrow.
func TestRenderPackageGraph_LinkStyleIndexIsStable(t *testing.T) {
	g := buildPackageGraph(t, fixturesDir+"/dependency-loops", false)

	got, err := New().RenderPackageGraph(g, render.Options{})
	if err != nil {
		t.Fatalf("RenderPackageGraph: %v", err)
	}

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	var arrowLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.Contains(trimmed, "-->") && !strings.HasPrefix(trimmed, "linkStyle ") {
			arrowLines = append(arrowLines, trimmed)
		}
	}

	if len(arrowLines) == 0 {
		t.Fatalf("no arrow lines found in output:\n%s", got)
	}
	if !strings.Contains(got, "linkStyle 0 stroke:red,stroke-width:4px") {
		t.Fatalf("output = %q, want a \"linkStyle 0 ...\" line targeting the mutual alpha<-->beta edge (index 0 once edges are sorted by From, To)", got)
	}
	if arrowLines[0] != "alpha <--> beta" {
		t.Errorf("first arrow line = %q, want %q (linkStyle 0 must point at this edge)", arrowLines[0], "alpha <--> beta")
	}
}

// TestRenderPackageGraph_RootPackage makes sure a root package (Dir
// ".") that both has children and participates in an edge renders
// without any subgraph/node ID collision: it appears as a plain node,
// never wrapped in a subgraph of its own.
func TestRenderPackageGraph_RootPackage(t *testing.T) {
	pkgs := []*gocode.Package{
		{Dir: ".", Name: "main", Imports: []gocode.Import{{Path: "example.com/root/child"}}},
		{Dir: "child", Name: "child"},
	}
	g := diagram.BuildPackageGraph(pkgs, "example.com/root", false)

	got, err := New().RenderPackageGraph(g, render.Options{})
	if err != nil {
		t.Fatalf("RenderPackageGraph: %v", err)
	}

	if !strings.Contains(got, `root["."]`) {
		t.Errorf("output = %q, want a root[\".\"] node line", got)
	}
	if !strings.Contains(got, "root --> child") {
		t.Errorf("output = %q, want a \"root --> child\" edge", got)
	}
	if strings.Contains(got, "subgraph root") {
		t.Errorf("output = %q, want no \"subgraph root\" (the root package is never wrapped in its own subgraph)", got)
	}
}
