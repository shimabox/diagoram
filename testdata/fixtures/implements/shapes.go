// Package implements is the "implements" fixture: it exercises Phase
// 5A's interface implementation heuristic in one place: direct
// implementation, one-level embedding promotion, a zero-method
// interface (excluded from detection), a same-named method with a
// mismatched signature (must not falsely match), a non-implementing
// interface, and (in the extras subpackage) a struct implementing an
// interface declared in another analyzed package.
package implements

// Named is implemented directly by Point and Square, and indirectly by
// Circle via Point's promoted Name().
type Named interface {
	Name() string
}

// Sized has no implementer in this fixture: nothing declares a
// matching Size() int method.
type Sized interface {
	Size() int
}

// Empty has zero methods, so it must be excluded from implementation
// detection entirely (otherwise every struct would trivially "match"
// it).
type Empty interface{}

// Describable is implemented by extras.Widget, a struct in another
// analyzed package, to exercise cross-package implementation
// detection: buildImplementationEdges checks every (struct, interface)
// pair in the whole analyzed set, not just same-package pairs, and
// does not require the two packages to import one another.
type Describable interface {
	Describe() string
}

// Labeled looks like Named at a glance (same method name, "Name") but
// its result type differs (int, not string): it must NOT match any
// struct's Name() string method, exercising signature comparison
// rather than a name-only check.
type Labeled interface {
	Name() int
}

// Point implements Named directly.
type Point struct {
	X, Y int
}

// Name returns a fixed label.
func (p Point) Name() string {
	return "point"
}

// Circle implements Named only through Point's promoted Name()
// (one-level struct embedding) and adds no Name() of its own.
type Circle struct {
	Point
	Radius int
}

// Diameter returns twice the radius.
func (c Circle) Diameter() int {
	return c.Radius * 2
}

// Square implements Named directly, with its own Name() (not via
// embedding).
type Square struct {
	Side int
}

// Name returns a fixed label.
func (s Square) Name() string {
	return "square"
}
