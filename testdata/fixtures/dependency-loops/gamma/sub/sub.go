// Package sub is a leaf package nested under gamma in the
// "dependency-loops" fixture, exercising subgraph nesting in the
// package diagram.
package sub

// Thing is sub's only declared type.
type Thing struct {
	Name string
}
