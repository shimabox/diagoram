package mermaid

import "testing"

func TestFormatTypeDistinguishesArrayAndSlice(t *testing.T) {
	if got, want := formatType("[16]byte"), "Array~16,byte~"; got != want {
		t.Errorf("formatType(array) = %q, want %q", got, want)
	}
	if got, want := formatType("[]byte"), "byte[]"; got != want {
		t.Errorf("formatType(slice) = %q, want %q", got, want)
	}
}
