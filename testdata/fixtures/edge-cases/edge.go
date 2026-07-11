// Package edge is the "edge-cases" fixture: pointers, slices, arrays,
// maps, generics, anonymous structs, and function-typed fields.
package edge

// Item is a referenced type used by Wrapper's composite fields.
type Item struct {
	ID int
}

type Key struct {
	Name string
}

// Wrapper wraps various composite TypeRef shapes.
type Wrapper struct {
	Ptr      *Item
	Slice    []Item
	PtrSlice []*Item
	Matrix   [3]int
	Lookup   map[string]*Item
	ByKey    map[Key]*Item
	Anon     struct {
		X int
	}
	Handler      func(int) error
	TypedHandler func(Key) Item
}

// Box is a generic container. Its type parameter must not be treated
// as a package dependency.
type Box[T any] struct {
	Value T
	Items []T
}

// Get returns the boxed value.
func (b Box[T]) Get() T {
	return b.Value
}
