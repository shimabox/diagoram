// Package extras is a sibling package in the "implements" fixture: its
// Widget implements the root package's Describable interface, without
// importing that package, to exercise cross-package implementation
// detection.
package extras

// Widget implements the root implements package's Describable.
type Widget struct {
	ID int
}

// Describe returns a fixed description.
func (w Widget) Describe() string {
	return "a widget"
}
