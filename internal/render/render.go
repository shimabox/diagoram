// Package render defines the Renderer interface that turns a
// diagram.Diagram into diagram text. It has no knowledge of any
// particular output format; format-specific renderers live in
// subpackages such as internal/render/mermaid.
package render

import "github.com/shimabox/diagoram/internal/diagram"

// Options controls renderer behavior. It is shared across output
// formats; fields that only apply to some formats are documented as
// such. It currently has no fields — filtering/visibility options
// (--hide-unexported, --disable-fields, ...) are added in later
// phases.
type Options struct{}

// Renderer turns a diagram.Diagram into its textual representation in
// some output format (e.g. Mermaid, PlantUML).
type Renderer interface {
	// Render returns d's textual representation, or an error if d
	// cannot be rendered under opt.
	Render(d *diagram.Diagram, opt Options) (string, error)
}
