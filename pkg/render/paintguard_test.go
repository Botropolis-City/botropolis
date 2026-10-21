package render

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item 195: the night's lights are cut by what the painter is told of,
// so a shape drawn around it, straight through the vector package, is
// a shape the windows behind it show through, as the district sign's
// board did. Only the night wash may: it is laid over the whole scene
// while the lights are noted, and must not cut them.
func TestNothingIsDrawnAroundThePainter(t *testing.T) {
	const vectorPath = "github.com/hajimehoshi/ebiten/v2/vector"
	allowed := map[string]bool{"wash": true}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	var around []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		file, err := parser.ParseFile(fset, name, src, 0)
		require.NoError(t, err)
		imported := ""
		for _, spec := range file.Imports {
			if p, _ := strconv.Unquote(spec.Path.Value); p == vectorPath {
				imported = "vector"
				if spec.Name != nil {
					imported = spec.Name.Name
				}
			}
		}
		if imported == "" {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || allowed[fn.Name.Name] {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == imported {
					around = append(around, fset.Position(call.Pos()).String()+": "+imported+"."+sel.Sel.Name)
				}
				return true
			})
		}
	}

	t.Run("when the render package's drawing is read", func(t *testing.T) {
		t.Run("it should draw nothing straight through the vector package but the night wash", func(t *testing.T) {
			assert.Empty(t, around)
		})
	})
}
