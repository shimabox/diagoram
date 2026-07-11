package gocode

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// parseTypeExpr parses src as a standalone type expression, as if it
// appeared in a field/parameter/result position.
func parseTypeExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	expr, err := parser.ParseExprFrom(token.NewFileSet(), "", src, 0)
	if err != nil {
		t.Fatalf("ParseExprFrom(%q): %v", src, err)
	}
	return expr
}

func TestTypeRefFromExpr(t *testing.T) {
	tests := []struct {
		src  string
		want TypeRef
	}{
		{
			src:  "int",
			want: TypeRef{Name: "int", String: "int"},
		},
		{
			src:  "string",
			want: TypeRef{Name: "string", String: "string"},
		},
		{
			src:  "User",
			want: TypeRef{Name: "User", String: "User"},
		},
		{
			src:  "*User",
			want: TypeRef{Name: "User", IsPtr: true, String: "*User"},
		},
		{
			src:  "[]User",
			want: TypeRef{Name: "User", IsSlice: true, String: "[]User"},
		},
		{
			src:  "[]*User",
			want: TypeRef{Name: "User", IsSlice: true, IsPtr: true, String: "[]*User"},
		},
		{
			src:  "*[]User",
			want: TypeRef{Name: "User", IsSlice: true, IsPtr: true, String: "*[]User"},
		},
		{
			src:  "[3]int",
			want: TypeRef{Name: "int", IsSlice: true, String: "[3]int"},
		},
		{
			src:  "map[string]int",
			want: TypeRef{Name: "int", IsMap: true, String: "map[string]int"},
		},
		{
			src:  "map[string]*User",
			want: TypeRef{Name: "User", IsMap: true, IsPtr: true, String: "map[string]*User"},
		},
		{
			src:  "map[string][]User",
			want: TypeRef{Name: "User", IsMap: true, IsSlice: true, String: "map[string][]User"},
		},
		{
			src:  "model.User",
			want: TypeRef{PkgName: "model", Name: "User", String: "model.User"},
		},
		{
			src:  "*model.User",
			want: TypeRef{PkgName: "model", Name: "User", IsPtr: true, String: "*model.User"},
		},
		{
			src:  "[]model.User",
			want: TypeRef{PkgName: "model", Name: "User", IsSlice: true, String: "[]model.User"},
		},
		{
			src:  "[]*model.User",
			want: TypeRef{PkgName: "model", Name: "User", IsSlice: true, IsPtr: true, String: "[]*model.User"},
		},
		{
			src:  "map[string]model.User",
			want: TypeRef{PkgName: "model", Name: "User", IsMap: true, String: "map[string]model.User"},
		},
		{
			src:  "map[model.Key]model.User",
			want: TypeRef{PkgName: "model", Name: "User", IsMap: true, String: "map[model.Key]model.User"},
		},
		{
			// Generic instantiation: type args are kept in String but
			// not used for dependency resolution.
			src:  "Box[int]",
			want: TypeRef{Name: "Box", String: "Box[int]"},
		},
		{
			src:  "[]Box[int]",
			want: TypeRef{Name: "Box", IsSlice: true, String: "[]Box[int]"},
		},
		{
			// Anonymous struct: no single Name, but String is populated.
			src:  "struct{ X int }",
			want: TypeRef{Name: "", String: "struct{ X int }"},
		},
		{
			// Function type: no single Name, but String is populated.
			src:  "func(int) error",
			want: TypeRef{Name: "", String: "func(int) error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			expr := parseTypeExpr(t, tt.src)
			got := typeRefFromExpr(expr)

			if got.PkgName != tt.want.PkgName {
				t.Errorf("PkgName = %q, want %q", got.PkgName, tt.want.PkgName)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.IsSlice != tt.want.IsSlice {
				t.Errorf("IsSlice = %v, want %v", got.IsSlice, tt.want.IsSlice)
			}
			if got.IsMap != tt.want.IsMap {
				t.Errorf("IsMap = %v, want %v", got.IsMap, tt.want.IsMap)
			}
			if got.IsPtr != tt.want.IsPtr {
				t.Errorf("IsPtr = %v, want %v", got.IsPtr, tt.want.IsPtr)
			}
			if got.String != tt.want.String {
				t.Errorf("String = %q, want %q", got.String, tt.want.String)
			}
		})
	}
}
