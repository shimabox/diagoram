package diagram

import (
	"strings"

	"github.com/shimabox/diagoram/internal/gocode"
)

// buildImplementationEdges heuristically detects Go's implicit
// interface satisfaction for every (concrete named type, interface) pair among
// pkgs: purely syntactic analysis (no go/types) cannot know for
// certain that a struct implements an interface, so this is a
// best-effort approximation. See the package doc below for the exact
// rule and the tradeoffs it makes.
//
// Rule:
//   - Interfaces with zero methods after embedding expansion (e.g.
//     "interface{}") are skipped entirely: every struct would
//     trivially "implement" them, which is never useful in a diagram.
//   - An interface's method set is its own declared Methods plus every
//     embedded interface's method set, expanded recursively (cycle-
//     safe) as far as embeds resolve to another analyzed interface;
//     an embed that does not resolve (external package, or not an
//     interface) is silently skipped rather than failing the whole
//     comparison.
//   - A struct's method set is its own declared Methods plus, for each
//     of its embedded fields, that field's own method set — but only
//     one level deep (the embedded type's own embeds are not
//     followed further). A struct's own method shadows a promoted one
//     of the same name, matching Go's actual promotion rules. One
//     level is enough for diagoram's purposes and keeps the walk
//     trivially cycle-free; see the Phase 5 plan for the rationale.
//   - A method "matches" when its name is identical and every
//     parameter/result type, compared position by position via
//     normalizeTypeRef, is equal. normalizeTypeRef intentionally
//     drops package qualification (comparing only the bare type name
//     plus pointer/slice/map shape): a purely syntactic analyzer
//     cannot always tell whether two differently-qualified names
//     denote the same type, and diagoram's stated policy is to accept
//     the resulting rare false positive rather than miss real
//     implementations over an import alias or a same-named type in
//     another package. A struct implements an interface when every
//     one of the interface's methods (after embedding expansion) has
//     a match in the struct's method set (also after embedding
//     expansion).
func buildImplementationEdges(pkgs []*gocode.Package, registry map[entryKey]*Entry, pkgByDir map[string]*gocode.Package, dirs []string, modulePath string) []Edge {
	ifaceByKey := map[entryKey]*gocode.Interface{}
	structByKey := map[entryKey]*gocode.Struct{}
	for _, pkg := range pkgs {
		for _, i := range pkg.Interfaces {
			ifaceByKey[entryKey{pkg.Dir, i.Name}] = i
		}
		for _, s := range pkg.Structs {
			structByKey[entryKey{pkg.Dir, s.Name}] = s
		}
	}

	ifaceMethodsMemo := map[entryKey][]gocode.Method{}
	var edges []Edge
	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			structEntry := registry[entryKey{pkg.Dir, s.Name}]
			structMethods := effectiveStructMethods(pkg.Dir, s, pkgByDir, dirs, modulePath, structByKey, ifaceByKey, ifaceMethodsMemo)

			for ifaceKey := range ifaceByKey {
				methods := resolveInterfaceMethods(ifaceKey, ifaceByKey, ifaceMethodsMemo, pkgByDir, dirs, modulePath, map[entryKey]bool{})
				if len(methods) == 0 {
					continue
				}
				if !implementsAll(structMethods, methods) {
					continue
				}
				ifaceEntry := registry[ifaceKey]
				edges = append(edges, Edge{From: structEntry.ID, To: ifaceEntry.ID, Kind: Implementation})
			}
		}
		for _, typ := range pkg.NamedTypes {
			concreteEntry := registry[entryKey{pkg.Dir, typ.Name}]
			concreteMethods := map[string]gocode.Method{}
			for _, method := range typ.Methods {
				concreteMethods[method.Name] = method
			}
			for ifaceKey := range ifaceByKey {
				methods := resolveInterfaceMethods(ifaceKey, ifaceByKey, ifaceMethodsMemo, pkgByDir, dirs, modulePath, map[entryKey]bool{})
				if len(methods) == 0 || !implementsAll(concreteMethods, methods) {
					continue
				}
				edges = append(edges, Edge{From: concreteEntry.ID, To: registry[ifaceKey].ID, Kind: Implementation})
			}
		}
	}
	return edges
}

// resolveInterfaceMethods returns key's full, flattened method set:
// its own declared Methods plus every embedded interface's method set
// (recursively). Results are memoized in memo; visiting guards against
// an embedding cycle (which would otherwise recurse forever) by
// treating a type already being resolved as contributing no further
// methods. An embed that does not resolve to another entry known in
// ifaceByKey (an external package, a struct, or a generic type
// constraint element) is silently skipped.
func resolveInterfaceMethods(key entryKey, ifaceByKey map[entryKey]*gocode.Interface, memo map[entryKey][]gocode.Method, pkgByDir map[string]*gocode.Package, dirs []string, modulePath string, visiting map[entryKey]bool) []gocode.Method {
	if m, ok := memo[key]; ok {
		return m
	}
	if visiting[key] {
		return nil
	}
	visiting[key] = true
	defer delete(visiting, key)

	iface, ok := ifaceByKey[key]
	if !ok {
		return nil
	}

	methods := append([]gocode.Method(nil), iface.Methods...)
	for _, embed := range iface.Embeds {
		embedKey, ok := resolveRefKey(pkgByDir, dirs, modulePath, key.Dir, embed)
		if !ok {
			continue
		}
		if _, ok := ifaceByKey[embedKey]; !ok {
			continue
		}
		methods = append(methods, resolveInterfaceMethods(embedKey, ifaceByKey, memo, pkgByDir, dirs, modulePath, visiting)...)
	}

	memo[key] = methods
	return methods
}

// effectiveStructMethods returns s's method set as a name -> Method
// map: s's own declared Methods, overlaid on top of every embedded
// field's own method set (one level deep only — see
// buildImplementationEdges' doc comment). An embedded field that
// resolves to another analyzed struct contributes that struct's own
// Methods; one that resolves to an analyzed interface contributes that
// interface's full (recursively expanded) method set, since Go
// promotes an embedded interface's methods to the outer struct just as
// it does an embedded struct's. An embed that does not resolve at all
// is silently skipped.
func effectiveStructMethods(dir string, s *gocode.Struct, pkgByDir map[string]*gocode.Package, dirs []string, modulePath string, structByKey map[entryKey]*gocode.Struct, ifaceByKey map[entryKey]*gocode.Interface, ifaceMethodsMemo map[entryKey][]gocode.Method) map[string]gocode.Method {
	methods := map[string]gocode.Method{}

	for _, embed := range s.Embeds {
		key, ok := resolveRefKey(pkgByDir, dirs, modulePath, dir, embed)
		if !ok {
			continue
		}
		if es, ok := structByKey[key]; ok {
			for _, m := range es.Methods {
				methods[m.Name] = m
			}
			continue
		}
		if _, ok := ifaceByKey[key]; ok {
			for _, m := range resolveInterfaceMethods(key, ifaceByKey, ifaceMethodsMemo, pkgByDir, dirs, modulePath, map[entryKey]bool{}) {
				methods[m.Name] = m
			}
		}
	}

	for _, m := range s.Methods {
		methods[m.Name] = m
	}
	return methods
}

// implementsAll reports whether structMethods (a name -> Method map)
// has, for every method in ifaceMethods, a same-named entry whose
// signature matches per signaturesMatch.
func implementsAll(structMethods map[string]gocode.Method, ifaceMethods []gocode.Method) bool {
	for _, im := range ifaceMethods {
		sm, ok := structMethods[im.Name]
		if !ok || !signaturesMatch(sm, im) {
			return false
		}
	}
	return true
}

// signaturesMatch reports whether a and b have the same number of
// parameters and results, with each position's normalized type (see
// normalizeTypeRef) equal.
func signaturesMatch(a, b gocode.Method) bool {
	if len(a.Params) != len(b.Params) || len(a.Results) != len(b.Results) {
		return false
	}
	for i := range a.Params {
		if normalizeTypeRef(a.Params[i]) != normalizeTypeRef(b.Params[i]) {
			return false
		}
	}
	for i := range a.Results {
		if normalizeTypeRef(a.Results[i]) != normalizeTypeRef(b.Results[i]) {
			return false
		}
	}
	return true
}

// normalizeTypeRef reduces a TypeRef to the text implementation
// detection compares: its pointer/slice/map shape plus its bare type
// name (ignoring any package qualifier). Types with no single name
// (anonymous structs, func types, channels, ...) fall back to their
// full String, which is safe since those never carry a package
// qualifier to begin with.
//
// Deliberately dropping the package qualifier is the "見逃しより誤検出
// を許容する" compromise the Phase 5 plan calls for: without go/types,
// diagoram cannot always tell whether "model.User" in one file and
// "User" (same package) or an aliased import in another denote the
// same type, so it accepts the rare false-positive Implementation edge
// in exchange for not missing real ones over that ambiguity.
func normalizeTypeRef(t gocode.TypeRef) string {
	name := t.Name
	if name == "" {
		name = t.String
	}

	var b strings.Builder
	if t.IsPtr {
		b.WriteByte('*')
	}
	if t.IsSlice {
		b.WriteString("[]")
	}
	if t.IsMap {
		b.WriteString("map[]")
	}
	b.WriteString(name)
	return b.String()
}
