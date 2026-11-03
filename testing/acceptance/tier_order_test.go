package acceptance_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/tools/acceptance-targets/targets"
)

// Item 330: the stress city's frames measure first in make
// test-performance, so nothing of the suite's own runs ahead of them in its
// process. go test runs a package's tests file by file in name order, and
// each file's in the order they are declared.
func TestTheStressCityMeasuresFirst(t *testing.T) {
	t.Run("when the performance tier is put in the order go test runs it", func(t *testing.T) {
		tests, err := targets.ReadTiers(".", measurePackage)
		require.NoError(t, err)
		files, err := filepath.Glob("*_test.go")
		require.NoError(t, err)
		sort.Strings(files)
		var order []string
		for _, file := range files {
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
			require.NoError(t, err)
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				if test, known := tests[fn.Name.Name]; known && test.Tier == targets.Performance {
					order = append(order, fn.Name.Name)
				}
			}
		}
		require.NotEmpty(t, order)

		t.Run("it should start with the stress city's frames", func(t *testing.T) {
			assert.Equal(t, "TestACityOfSixHundredDistrictsFrames", order[0], "the tier runs %v", order)
		})
	})
}
