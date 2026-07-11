// Package shape is the "interfaces" fixture: interface declarations,
// interface embedding, and struct embedding.
package shape

// Named provides a display name.
type Named interface {
	Name() string
}

// Shape describes a 2D shape. It embeds Named.
type Shape interface {
	Named
	Area() float64
}

// Base provides a Name() method via an embedded field.
type Base struct {
	Label string
}

// Name returns the label.
func (b Base) Name() string {
	return b.Label
}

// Circle is a shape that satisfies Shape by embedding Base and adding
// Area().
type Circle struct {
	Base
	Radius float64
}

// Area returns the circle's area.
func (c Circle) Area() float64 {
	return 3.14159 * c.Radius * c.Radius
}
