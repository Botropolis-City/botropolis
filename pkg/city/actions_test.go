package city_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// declaredActions reads every ActionKind constant out of the package's
// source, so an action added without a description fails here rather
// than relying on a list someone has to remember to extend.
func declaredActions(t *testing.T) []city.ActionKind {
	t.Helper()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	var kinds []city.ActionKind
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok || spec.Type == nil {
				return true
			}
			if id, ok := spec.Type.(*ast.Ident); !ok || id.Name != "ActionKind" {
				return true
			}
			for _, v := range spec.Values {
				if lit, ok := v.(*ast.BasicLit); ok {
					s, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					if s != "" {
						kinds = append(kinds, city.ActionKind(s))
					}
				}
			}
			return true
		})
	}
	require.NotEmpty(t, kinds, "no action kinds found: the check below passes on nothing")
	return kinds
}

func TestActionDescriptions(t *testing.T) {
	t.Run("for every action kind", func(t *testing.T) {
		for _, k := range declaredActions(t) {
			t.Run("when it is "+string(k), func(t *testing.T) {
				t.Run("it should describe what it does in one line", func(t *testing.T) {
					d := k.Describe()
					assert.True(t, d != "" && !strings.Contains(d, "\n"), "%q", d)
				})
			})
		}
	})
}
