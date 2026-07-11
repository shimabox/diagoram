// Package gamma is part of the "dependency-loops" fixture: an
// ordinary, non-cyclic dependency of alpha that itself depends on the
// nested package gamma/sub, exercising subgraph nesting in the
// package diagram.
package gamma

import "example.com/looptest/gamma/sub"

// Thing is gamma's only declared type.
type Thing struct {
	S sub.Thing
}
