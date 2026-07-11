package gocode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Parse analyzes every Go package under rootDir and returns the
// resulting language model, in deterministic order: packages sorted
// by Dir, and each package's declarations/imports sorted (by
// source order, or by Path/Alias for Imports).
//
// Directories named "vendor" or "testdata", and any directory whose
// base name starts with ".", are skipped. Within each remaining
// directory, only files matching opt.Includes (default ["*.go"]) and
// not matching opt.Excludes (default ["*_test.go"]) are analyzed.
//
// Files that fail to parse (syntax errors) are skipped rather than
// aborting the analysis; they are reported in the returned []Warning
// instead. Parse only returns a non-nil error for problems unrelated
// to individual source files, such as rootDir being unreadable.
func Parse(rootDir string, opt ParseOptions) ([]*Package, []Warning, error) {
	dirs, err := discoverDirs(rootDir, opt)
	if err != nil {
		return nil, nil, err
	}

	var packages []*Package
	var warnings []Warning

	for _, d := range dirs {
		pkg, warns := parseDir(d)
		warnings = append(warnings, warns...)
		if pkg != nil {
			packages = append(packages, pkg)
		}
	}

	return packages, warnings, nil
}

// parseDir parses every file in d and assembles them into a Package.
// It returns (nil, warnings) if every file in the directory failed to
// parse.
func parseDir(d dirFiles) (*Package, []Warning) {
	fset := token.NewFileSet()

	var warnings []Warning
	var files []*ast.File
	pkgName := ""

	for _, name := range d.Files {
		relPath := filepath.ToSlash(filepath.Join(d.Dir, name))
		absPath := filepath.Join(d.AbsDir, name)

		file, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
		if err != nil {
			warnings = append(warnings, Warning{File: relPath, Err: err})
			continue
		}
		if pkgName == "" {
			pkgName = file.Name.Name
		} else if file.Name.Name != pkgName {
			warnings = append(warnings, Warning{
				File: relPath,
				Err:  fmt.Errorf("package %q does not match package %q already selected for this directory", file.Name.Name, pkgName),
			})
			continue
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		return nil, warnings
	}

	pkg := &Package{Dir: d.Dir, Name: pkgName}
	methodsByReceiver := map[string][]Method{}
	methodSets := map[string]map[string]bool{}
	constantsByType := map[string][]Constant{}
	var pendingConstants []pendingConstant
	importSet := map[Import]bool{}
	functionSet := map[string]bool{}

	for _, file := range files {
		fd := collectDecls(file)
		pkg.Structs = append(pkg.Structs, fd.Structs...)
		pkg.Interfaces = append(pkg.Interfaces, fd.Interfaces...)
		pkg.NamedTypes = append(pkg.NamedTypes, fd.NamedTypes...)
		for _, function := range fd.Functions {
			key := functionSignatureKey(function)
			if !functionSet[key] {
				functionSet[key] = true
				pkg.Functions = append(pkg.Functions, function)
			}
		}
		for recv, methods := range fd.Methods {
			if methodSets[recv] == nil {
				methodSets[recv] = map[string]bool{}
			}
			for _, method := range methods {
				key := signatureKey(method.Name, method.Params, method.Results)
				if !methodSets[recv][key] {
					methodSets[recv][key] = true
					methodsByReceiver[recv] = append(methodsByReceiver[recv], method)
				}
			}
		}
		for typeName, constants := range fd.Constants {
			constantsByType[typeName] = append(constantsByType[typeName], constants...)
		}
		pendingConstants = append(pendingConstants, fd.PendingConstants...)

		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				path = spec.Path.Value
			}
			alias := ""
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			importSet[Import{Alias: alias, Path: path}] = true
		}
	}

	for _, s := range pkg.Structs {
		s.Methods = methodsByReceiver[s.Name]
	}
	for _, typ := range pkg.NamedTypes {
		typ.Methods = methodsByReceiver[typ.Name]
		typ.Constants = constantsByType[typ.Name]
	}
	resolvePendingConstants(pkg.NamedTypes, constantsByType, pendingConstants)

	pkg.Imports = make([]Import, 0, len(importSet))
	for imp := range importSet {
		pkg.Imports = append(pkg.Imports, imp)
	}
	sort.Slice(pkg.Imports, func(i, j int) bool {
		if pkg.Imports[i].Path != pkg.Imports[j].Path {
			return pkg.Imports[i].Path < pkg.Imports[j].Path
		}
		return pkg.Imports[i].Alias < pkg.Imports[j].Alias
	})

	return pkg, warnings
}

func functionSignatureKey(function Function) string {
	return signatureKey(function.Name, function.Params, function.Results)
}

func signatureKey(name string, params, results []TypeRef) string {
	var key strings.Builder
	key.WriteString(name)
	for _, ref := range params {
		key.WriteByte('\x00')
		key.WriteString(ref.String)
	}
	key.WriteByte('\x01')
	for _, ref := range results {
		key.WriteByte('\x00')
		key.WriteString(ref.String)
	}
	return key.String()
}

func resolvePendingConstants(namedTypes []*NamedType, constantsByType map[string][]Constant, pending []pendingConstant) {
	typeByConstant := map[string]string{}
	for typeName, constants := range constantsByType {
		for _, constant := range constants {
			typeByConstant[constant.Name] = typeName
		}
	}
	for len(pending) > 0 {
		var unresolved []pendingConstant
		progress := false
		for _, item := range pending {
			typeName, ok := typeByConstant[item.RefName]
			if !ok {
				unresolved = append(unresolved, item)
				continue
			}
			constantsByType[typeName] = append(constantsByType[typeName], item.Constant)
			typeByConstant[item.Constant.Name] = typeName
			progress = true
		}
		if !progress {
			break
		}
		pending = unresolved
	}
	for _, typ := range namedTypes {
		typ.Constants = constantsByType[typ.Name]
	}
}
