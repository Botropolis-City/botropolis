package render

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// textFields is every type in pkg/render that holds a field, and whether
// the keyboard reaches it: one of its methods updates a textinput.Composer
// it holds, or a function that calls ebiten.AppendInputChars touches the
// field. A field fed only the scripted
// rune passes every scripted test and takes no typing. Item 289.
// pasting is whether a function that calls g.pasted touches the field,
// which is the one way a paste reaches one. Item 290.
func textFields(t *testing.T) (holders, pasting map[string]bool) {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	holders, pasting = map[string]bool{}, map[string]bool{}
	fieldNames := map[string]map[string]bool{}
	composers := map[string]map[string]bool{}
	var funcs []*ast.FuncDecl
	note := func(m map[string]map[string]bool, typ string, names []*ast.Ident) {
		if m[typ] == nil {
			m[typ] = map[string]bool{}
		}
		for _, name := range names {
			m[typ][name.Name] = true
		}
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSpec:
				st, ok := n.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, f := range st.Fields.List {
					if isField(f.Type) {
						holders[n.Name.Name] = false
						note(fieldNames, n.Name.Name, f.Names)
					}
					if isComposer(f.Type) {
						note(composers, n.Name.Name, f.Names)
					}
				}
			case *ast.FuncDecl:
				if n.Body != nil {
					funcs = append(funcs, n)
				}
			}
			return true
		})
	}
	for _, fn := range funcs {
		var appends, pastes bool
		updated, touched := map[string]bool{}, map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			touched[sel.Sel.Name] = true
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "ebiten" && sel.Sel.Name == "AppendInputChars" {
				appends = true
			}
			if sel.Sel.Name == "pasted" {
				pastes = true
			}
			if inner, ok := sel.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Update" {
				updated[inner.Sel.Name] = true
			}
			return true
		})
		for typ := range holders {
			for name := range composers[typ] {
				if updated[name] && fn.Recv != nil && receiverName(fn.Recv.List[0].Type) == typ {
					holders[typ] = true
				}
			}
			for name := range fieldNames[typ] {
				if appends && touched[name] {
					holders[typ] = true
				}
				if pastes && touched[name] {
					pasting[typ] = true
				}
			}
		}
	}
	return holders, pasting
}

func isField(e ast.Expr) bool {
	if arr, ok := e.(*ast.ArrayType); ok {
		e = arr.Elt
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "field"
}

func isComposer(e ast.Expr) bool {
	star, ok := e.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "textinput" && sel.Sel.Name == "Composer"
}

func receiverName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func TestEveryTextFieldReadsTheKeyboard(t *testing.T) {
	t.Run("when pkg/render's source is read", func(t *testing.T) {
		holders, pasting := textFields(t)
		var names []string
		for name := range holders {
			names = append(names, name)
		}
		sort.Strings(names)

		t.Run("it should find the finder, New session and triage among the types holding a field", func(t *testing.T) {
			assert.Subset(t, names, []string{"finder", "newSession", "snoozePicker"})
		})

		for _, name := range names {
			t.Run("and "+name+" holds a field", func(t *testing.T) {
				t.Run("it should read the keyboard, not only the scripted rune", func(t *testing.T) {
					assert.True(t, holders[name], "%s holds a text field but neither updates a textinput.Composer nor calls ebiten.AppendInputChars", name)
				})

				t.Run("it should take a paste through g.pasted", func(t *testing.T) {
					assert.True(t, pasting[name], "%s holds a text field that no function calling g.pasted touches", name)
				})
			})
		}
	})
}
