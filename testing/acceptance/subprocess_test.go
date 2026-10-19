package acceptance_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFile struct {
	path string
	fset *token.FileSet
	file *ast.File
}

func parseTests(t *testing.T, dirs ...string) []testFile {
	t.Helper()
	var files []testFile
	for _, dir := range dirs {
		require.NoError(t, filepath.WalkDir(filepath.Join("..", "..", dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			files = append(files, testFile{p, fset, f})
			return nil
		}))
	}
	return files
}

func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func execCalls(f *ast.File, visit func(call *ast.CallExpr)) {
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Command" {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "exec" && len(call.Args) > 0 {
				visit(call)
			}
		}
		return true
	})
}

func namesTools(s string) bool {
	return s == "tools" || strings.Contains(s, "tools/")
}

func bareToolExecs(files []testFile) []string {
	consts := map[string]bool{}
	for _, tf := range files {
		for _, decl := range tf.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, v := range vs.Values {
					if s, ok := literal(v); ok && namesTools(s) {
						consts[vs.Names[i].Name] = true
					}
				}
			}
		}
	}
	var found []string
	for _, tf := range files {
		execCalls(tf.file, func(call *ast.CallExpr) {
			ast.Inspect(call.Args[0], func(n ast.Node) bool {
				bare := false
				if e, ok := n.(ast.Expr); ok {
					if s, ok := literal(e); ok && namesTools(s) {
						bare = true
					}
				}
				if id, ok := n.(*ast.Ident); ok && consts[id.Name] {
					bare = true
				}
				if bare {
					found = append(found, tf.fset.Position(call.Pos()).String())
				}
				return !bare
			})
		})
	}
	sort.Strings(found)
	return found
}

func buildsWithoutTheEntryPoint(files []testFile) []string {
	builds := map[string]string{}
	links := map[string]bool{}
	for _, tf := range files {
		dir := filepath.Dir(tf.path)
		execCalls(tf.file, func(call *ast.CallExpr) {
			if len(call.Args) < 2 {
				return
			}
			first, _ := literal(call.Args[0])
			second, _ := literal(call.Args[1])
			if first != "go" || second != "build" {
				return
			}
			for _, a := range call.Args[2:] {
				if s, ok := literal(a); ok && strings.HasSuffix(s, "cmd/botropolis") {
					builds[dir] = tf.fset.Position(call.Pos()).String()
				}
			}
		})
		app := ""
		for _, imp := range tf.file.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "botropolis.city/botropolis/pkg/app" {
				app = "app"
				if imp.Name != nil {
					app = imp.Name.Name
				}
			}
		}
		if app == "" {
			continue
		}
		ast.Inspect(tf.file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Main" {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == app {
					links[dir] = true
				}
			}
			return true
		})
	}
	var found []string
	for dir, at := range builds {
		if !links[dir] {
			found = append(found, fmt.Sprintf("%s builds cmd/botropolis but its tests never reference app.Main", at))
		}
	}
	sort.Strings(found)
	return found
}

func TestEveryBuiltBinaryIsInTheCacheKey(t *testing.T) {
	t.Run("when every test that go-builds cmd/botropolis is parsed", func(t *testing.T) {
		found := buildsWithoutTheEntryPoint(parseTests(t, "cmd", "pkg", "testing"))

		t.Run("it should find each one's package referencing app.Main", func(t *testing.T) {
			assert.Empty(t, found, "reference app.Main in that package, as testing/acceptance/binary_test.go does, so the test cache keys on the code the binary runs")
		})
	})
}

func TestScriptsAreInTheCacheKey(t *testing.T) {
	t.Run("when every exec.Command in the acceptance tests is parsed", func(t *testing.T) {
		found := bareToolExecs(parseTests(t, "testing/acceptance"))

		t.Run("it should name no tools/ script except through helpers.Tool", func(t *testing.T) {
			assert.Empty(t, found, "run tools/ scripts through helpers.Tool, which opens them so the test cache keys on them")
		})
	})

	for _, c := range []cacheRun{
		{
			name: "the acceptance tests' TestDemoCorpusIsClean", pkg: "./testing/acceptance", run: "^TestDemoCorpusIsClean$",
			extra:  []string{"demo", "tools"},
			probes: []probe{appendProbe("in tools/check-demo-corpus", "tools/check-demo-corpus")},
		},
		{
			name: "the acceptance tests' TestAssetSources", pkg: "./testing/acceptance", run: "^TestAssetSources$",
			extra:  []string{"tools"},
			probes: []probe{appendProbe("in tools/check-asset-sources", "tools/check-asset-sources")},
		},
		{
			name: "the packaging tests' TestShellIntegration", pkg: "./testing/integration/packaging", run: "^TestShellIntegration$",
			extra:  []string{"packaging"},
			probes: []probe{appendProbe("in the shim packaging/botropolis.bash", "packaging/botropolis.bash")},
		},
	} {
		c.assert(t)
	}
}
