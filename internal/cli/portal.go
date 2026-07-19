package cli

import (
	"fmt"
	"io"

	"github.com/shimabox/diagoram/internal/diagram"
	"github.com/shimabox/diagoram/internal/gocode"
	"github.com/shimabox/diagoram/internal/portal"
	"github.com/shimabox/diagoram/internal/render"
	"github.com/shimabox/diagoram/internal/render/mermaid"
	"github.com/shimabox/diagoram/internal/render/plantuml"
)

// runPortal implements --html: it builds the same class diagram and
// package graph Run's other output modes build, applying the same
// filters in the same order as Run's own class-diagram branch (see
// that branch's comment for why this ~15-line duplication is
// intentional rather than factored out), renders all four diagram
// texts plus a summary and Markdown report by reusing existing
// renderers and markdownReport, and hands the result to
// portal.Generate.
func runPortal(opts *Options, pkgs []*gocode.Package, warnings []gocode.Warning, modulePath string, parseOptions gocode.ParseOptions, stdout, stderr io.Writer) int {
	d := diagram.BuildWithModulePath(pkgs, modulePath)
	if opts.HideUnexported {
		d = diagram.FilterUnexported(d)
	}
	if opts.HideAliases {
		d = diagram.FilterAliases(d)
	}
	if len(opts.RelTargets) > 0 {
		filtered, filterErr := diagram.FilterByRelTarget(d, opts.RelTargets, opts.RelTargetDepth)
		if filterErr != nil {
			fmt.Fprintf(stderr, "Error: %v\n", filterErr)
			return 1
		}
		d = filtered
	}
	g := diagram.BuildPackageGraph(pkgs, modulePath, opts.ShowExternal)

	renderOptions := render.Options{
		HideUnexported:    opts.HideUnexported,
		ShowConstants:     opts.ShowConstants,
		ShowFunctions:     opts.ShowFunctions,
		DisableFields:     opts.DisableFields,
		DisableMethods:    opts.DisableMethods,
		DisableImplements: opts.DisableImplements,
		FunctionPatterns:  opts.FunctionPatterns,
		MethodPatterns:    opts.MethodPatterns,
		ReceiverPatterns:  opts.ReceiverPatterns,
		MaxMembers:        opts.MaxMembers,
		ShowEdgeReasons:   opts.ShowEdgeReasons,
	}

	classMermaid, err := mermaid.New().Render(d, renderOptions)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
		return 1
	}
	packageMermaid, err := mermaid.New().RenderPackageGraph(g, render.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
		return 1
	}
	classPlantUML, err := plantuml.New().Render(d, renderOptions)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
		return 1
	}
	packagePlantUML, err := plantuml.New().RenderPackageGraph(g, render.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render diagram: %v\n", err)
		return 1
	}

	summary := diagram.Summary(d, diagram.SummaryOptions{
		HideUnexported:    opts.HideUnexported,
		DisableFields:     opts.DisableFields,
		DisableMethods:    opts.DisableMethods,
		DisableImplements: opts.DisableImplements,
		FunctionPatterns:  opts.FunctionPatterns,
		MethodPatterns:    opts.MethodPatterns,
		ReceiverPatterns:  opts.ReceiverPatterns,
		ShowEdgeReasons:   opts.ShowEdgeReasons,
	})

	report, err := markdownReport(opts, d, warnings, modulePath, parseOptions.BuildContext)
	if err != nil {
		fmt.Fprintf(stderr, "Error: cannot render report: %v\n", err)
		return 1
	}

	artifacts := portal.Artifacts{
		ClassMermaid:    classMermaid,
		PackageMermaid:  packageMermaid,
		ClassPlantUML:   classPlantUML,
		PackagePlantUML: packagePlantUML,
		Summary:         summary,
		ReportMarkdown:  report,
	}
	meta := portal.Meta{
		Dir:        opts.Dir,
		ModulePath: modulePath,
		Version:    version,
	}

	result, genErr := portal.Generate(opts.HTMLDir, artifacts, meta)
	if genErr != nil {
		fmt.Fprintf(stderr, "Error: cannot write HTML portal to %q: %v\n", opts.HTMLDir, genErr)
		return 1
	}

	fmt.Fprintf(stdout, "Portal written to %s\n", result.IndexPath)
	return 0
}
