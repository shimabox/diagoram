// Package render defines the Renderer interface that turns a
// diagram.Diagram into diagram text. It has no knowledge of any
// particular output format; format-specific renderers live in
// subpackages such as internal/render/mermaid.
package render

import "github.com/shimabox/diagoram/internal/diagram"

// Options controls renderer behavior. It is shared across output
// formats; fields that only apply to some formats are documented as
// such.
type Options struct {
	// HideUnexported drops unexported types, fields, and methods from the
	// rendered diagram (--hide-unexported).
	HideUnexported bool
	// ShowConstants includes constants associated with named types.
	ShowConstants bool
	// ShowFunctions includes synthetic package-level function entries.
	ShowFunctions bool
	// DisableFields omits every Entry's fields from the rendered class
	// body (--disable-fields).
	DisableFields bool
	// DisableMethods omits every Entry's methods from the rendered
	// class body (--disable-methods).
	DisableMethods bool
	// DisableImplements omits diagram.Implementation edges from the
	// rendered diagram (--disable-implements), for projects where the
	// heuristic produces too many arrows to read comfortably.
	DisableImplements bool
}

// Renderer turns a diagram.Diagram into its textual representation in
// some output format (e.g. Mermaid, PlantUML).
type Renderer interface {
	// Render returns d's textual representation, or an error if d
	// cannot be rendered under opt.
	Render(d *diagram.Diagram, opt Options) (string, error)
}

// PackageGraphRenderer is implemented by every Renderer that can also
// render a diagram.PackageGraph (--package-diagram). It is a separate
// interface, rather than a method on Renderer itself, so that a future
// output format could in principle support only one diagram kind; both
// of diagoram's current renderers (mermaid, plantuml) implement it.
type PackageGraphRenderer interface {
	Renderer
	// RenderPackageGraph returns g's textual representation, or an
	// error if g cannot be rendered under opt.
	RenderPackageGraph(g *diagram.PackageGraph, opt Options) (string, error)
}
