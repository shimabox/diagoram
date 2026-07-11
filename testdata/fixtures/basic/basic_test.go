package product

// This file exists only to verify that the analyzer's default
// exclusion of *_test.go files works against the "basic" fixture: it
// declares a type that must NOT show up in Parse results by default.

type ShouldBeExcludedByDefault struct {
	X int
}
