package acceptance_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const xvfbRun = "xvfb" + "-run"

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func walkRepo(t *testing.T, root string, visit func(rel, path string)) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "bin", "dist", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		visit(rel, path)
		return nil
	}))
}

// Item 126: xvfb-run -a picks a display by looking for a free one, so
// runs started together collide. pkg/xvfb has the server report the
// display it took; nothing else may start a virtual display another way.
func TestNothingRunsXvfbRun(t *testing.T) {
	root := repoRoot(t)
	_, self, _, _ := runtime.Caller(0)

	t.Run("when every Go file's string literals are read", func(t *testing.T) {
		var found []string
		walkRepo(t, root, func(rel, path string) {
			if !strings.HasSuffix(path, ".go") || filepath.Base(path) == filepath.Base(self) {
				return
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			require.NoError(t, err, rel)
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if s, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(s, xvfbRun) {
					found = append(found, fset.Position(lit.Pos()).String())
				}
				return true
			})
		})

		t.Run("it should find none naming xvfb-run", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})

	t.Run("when the Makefile, tools and workflows are read, comments skipped", func(t *testing.T) {
		var found []string
		walkRepo(t, root, func(rel, path string) {
			top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
			if rel != "Makefile" && top != "tools" && !strings.HasPrefix(filepath.ToSlash(rel), ".github/workflows/") {
				return
			}
			data, err := os.ReadFile(path)
			require.NoError(t, err, rel)
			for i, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				if strings.Contains(line, xvfbRun) {
					found = append(found, rel+":"+strconv.Itoa(i+1))
				}
			}
		})

		t.Run("it should find no call to xvfb-run", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})
}

// Item 186: Xvfb gives the next server the display number just freed, and
// a server started without a cookie answers anyone, so the hotkey test's
// open server answered the xvfb test's check that a closed server had
// stopped. pkg/xvfb starts every server with a cookie; nothing else may
// start one.
func TestOnlyPkgXvfbStartsXvfb(t *testing.T) {
	root := repoRoot(t)

	t.Run("when every Go file's exec.Command calls are read", func(t *testing.T) {
		var calls int
		var found []string
		walkRepo(t, root, func(rel, path string) {
			if !strings.HasSuffix(path, ".go") {
				return
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			require.NoError(t, err, rel)
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
					return true
				}
				calls++
				for _, arg := range call.Args {
					lit, ok := arg.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if s, err := strconv.Unquote(lit.Value); err == nil && filepath.Base(s) == "Xvfb" && filepath.ToSlash(filepath.Dir(rel)) != "pkg/xvfb" {
						found = append(found, fset.Position(lit.Pos()).String())
					}
				}
				return true
			})
		})
		require.NotZero(t, calls)

		t.Run("it should find none starting Xvfb outside pkg/xvfb", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})
}
