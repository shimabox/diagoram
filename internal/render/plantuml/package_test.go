package plantuml

import (
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
// fixture, the same one internal/render/mermaid's own equivalent test
// uses: a direct import cycle rendered as a red, bold, bidirectional
// arrow, plus a nested package and an ordinary one-way edge.
func TestRenderPackageGraph_GoldenFixture(t *testing.T) {
	dir := fixturesDir + "/dependency-loops"

	t.Run("default hides external packages", func(t *testing.T) {
		g := buildPackageGraph(t, dir, false)

		got, err := New().RenderPackageGraph(g, render.Options{})
		if err != nil {
			t.Fatalf("RenderPackageGraph: %v", err)
		}

		testutil.Golden(t, dir+"/expected-package.puml", got)
	})

	t.Run("show-external includes fmt", func(t *testing.T) {
		g := buildPackageGraph(t, dir, true)

		got, err := New().RenderPackageGraph(g, render.Options{})
		if err != nil {
			t.Fatalf("RenderPackageGraph: %v", err)
		}

		testutil.Golden(t, dir+"/expected-package-external.puml", got)
	})
}

// TestRenderPackageGraph_Empty makes sure an empty PackageGraph
// renders to a minimal, valid PlantUML script with no trailing
// garbage.
func TestRenderPackageGraph_Empty(t *testing.T) {
	got, err := New().RenderPackageGraph(diagram.BuildPackageGraph(nil, "", false), render.Options{})
	if err != nil {
		t.Fatalf("RenderPackageGraph: %v", err)
	}
	want := "@startuml package-related-diagram\n@enduml\n"
	if got != want {
		t.Errorf("RenderPackageGraph(empty) = %q, want %q", got, want)
	}
}

// TestRenderPackageGraph_RootPackage makes sure a root package (Dir
// ".") that both has children and participates in an edge renders as
// a plain "root" alias, never wrapped in its own package block (there
// is no PackageNode for it to nest inside), mirroring
// internal/render/mermaid's own TestRenderPackageGraph_RootPackage.
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

	want := "@startuml package-related-diagram\n" +
		"    package \".\" as root\n" +
		"    package child as child\n" +
		"    root --> child\n" +
		"@enduml\n"
	if got != want {
		t.Errorf("RenderPackageGraph() = %q, want %q", got, want)
	}
}
