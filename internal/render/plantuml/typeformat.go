package plantuml

import "strings"

// safeType turns a gocode.TypeRef.String value (Go source notation,
// e.g. "[]*model.User", "map[string]int", "Box[int]",
// "func(int) error") into text that is safe to place on a single
// PlantUML class member line.
//
// Unlike Mermaid, PlantUML's classDiagram grammar does not give
// special meaning to "[", "]", "(", or ")" inside a field's type text
// (a member is only ever parsed as a method when "(" immediately
// follows its name, and diagoram always separates a field's name from
// its type with " : ", never placing a type directly after the name),
// so most of Go's own type notation can be emitted unchanged.
//
// The one thing that cannot be emitted unchanged is a type whose
// go/printer text spans multiple source lines: go/printer renders an
// anonymous struct or interface literal with two or more members
// across multiple lines (e.g. "struct {\n\tA int\n\tB string\n}"), and
// an embedded raw newline would corrupt PlantUML's line-oriented
// class-body syntax. safeType collapses any run of whitespace
// (including newlines and the tabs go/printer aligns struct fields
// with) into a single space, which is a no-op for every type that was
// already single-line and makes every other type safe to embed.
func safeType(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
