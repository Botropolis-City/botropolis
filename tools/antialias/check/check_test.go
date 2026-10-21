package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const vectorPath = "github.com/hajimehoshi/ebiten/v2/vector"

// vectorSource stands in for the vector package: three functions with a
// trailing antialias bool, one of them a name no hand list would carry,
// and two that are not anti-aliasing switches.
const vectorSource = `package vector

import "image/color"

func FillCircle(dst any, cx, cy, r float32, clr color.Color, antialias bool) {}
func StrokeLine(dst any, x0, y0, x1, y1, w float32, clr color.Color, antialias bool) {}
func FillHeart(dst any, x, y float32, clr color.Color, antialias bool) {}
func FillPath(dst any, path any, fill any, op any) {}
func Keep(dst any, keep bool) {}
func fillInternal(dst any, antialias bool) {}
`

type fixture struct {
	root, vector string
}

func newFixture(t *testing.T, files map[string]string) fixture {
	t.Helper()
	f := fixture{root: t.TempDir(), vector: t.TempDir()}
	require.NoError(t, os.WriteFile(filepath.Join(f.vector, "vector.go"), []byte(vectorSource), 0o644))
	for name, src := range files {
		path := filepath.Join(f.root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	}
	return f
}

func (f fixture) run(t *testing.T, allow string) ([]string, error) {
	t.Helper()
	findings, err := Run(f.root, f.vector, vectorPath, strings.NewReader(allow))
	var out []string
	for _, x := range findings {
		out = append(out, x.String())
	}
	return out, err
}

func source(body string) string {
	return fmt.Sprintf("package draw\n\nimport \"%s\"\n\nfunc draw(dst any) {\n%s\n}\n", vectorPath, body)
}

func TestRun(t *testing.T) {
	for _, fn := range []string{"FillCircle", "StrokeLine", "FillHeart"} {
		t.Run("when a file calls vector."+fn+" with anti-aliasing on", func(t *testing.T) {
			args := map[string]string{
				"FillCircle": "dst, 1, 2, 3, nil, true",
				"StrokeLine": "dst, 1, 2, 3, 4, 5, nil, true",
				"FillHeart":  "dst, 1, 2, nil, true",
			}[fn]
			f := newFixture(t, map[string]string{"pkg/draw/draw.go": source("\tvector." + fn + "(" + args + ")")})
			found, err := f.run(t, "")
			require.NoError(t, err)

			t.Run("it should report it, naming the file and line", func(t *testing.T) {
				assert.Equal(t, []string{"pkg/draw/draw.go:6: vector." + fn + " with anti-aliasing on"}, found)
			})
		})
	}

	reported := []struct{ when, body string }{
		{"a file sets AntiAlias: true in an options literal", "\top := &vector.DrawPathOptions{AntiAlias: true}\n\t_ = op"},
		{"a file sets op.AntiAlias = true", "\tvar op vector.DrawPathOptions\n\top.AntiAlias = true"},
		{"a call passes anti-aliasing as a variable", "\taa := false\n\tvector.FillCircle(dst, 1, 2, 3, nil, aa)"},
	}
	for _, c := range reported {
		t.Run("when "+c.when, func(t *testing.T) {
			f := newFixture(t, map[string]string{"pkg/draw/draw.go": source(c.body)})
			found, err := f.run(t, "")
			require.NoError(t, err)

			t.Run("it should report it", func(t *testing.T) {
				assert.Len(t, found, 1)
			})
		})
	}

	t.Run("when the vector package is imported under another name", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": "package draw\n\nimport v \"" + vectorPath + "\"\n\nfunc draw(dst any) {\n\tv.FillCircle(dst, 1, 2, 3, nil, true)\n}\n"})
		found, err := f.run(t, "")
		require.NoError(t, err)

		t.Run("it should still report the call", func(t *testing.T) {
			assert.Equal(t, []string{"pkg/draw/draw.go:6: vector.FillCircle with anti-aliasing on"}, found)
		})
	})

	t.Run("when another package named vector has the same function", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": "package draw\n\nimport \"example.com/vector\"\n\nfunc draw(dst any) {\n\tvector.FillCircle(dst, 1, 2, 3, nil, true)\n}\n"})
		found, err := f.run(t, "")
		require.NoError(t, err)

		t.Run("it should report nothing", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})

	t.Run("when an anti-aliased call is in a package the guard has never seen", func(t *testing.T) {
		f := newFixture(t, map[string]string{
			"pkg/draw/draw.go":           source("\tvector.FillCircle(dst, 1, 2, 3, nil, false)"),
			"cmd/somewhere/new/paint.go": source("\tvector.StrokeLine(dst, 1, 2, 3, 4, 5, nil, true)"),
		})
		found, err := f.run(t, "")
		require.NoError(t, err)

		t.Run("it should report it", func(t *testing.T) {
			assert.Equal(t, []string{"cmd/somewhere/new/paint.go:6: vector.StrokeLine with anti-aliasing on"}, found)
		})
	})

	t.Run("when every call has anti-aliasing off", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": source("\tvector.FillCircle(dst, 1, 2, 3, nil, false)\n\tvector.Keep(dst, true)\n\top := &vector.DrawPathOptions{AntiAlias: false}\n\t_ = op")})
		found, err := f.run(t, "")
		require.NoError(t, err)

		t.Run("it should report nothing", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})

	t.Run("when the call is inside an allowlisted function", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": "package draw\n\nimport \"" + vectorPath + "\"\n\ntype Painter struct{}\n\nfunc (p *Painter) stamp(dst any) {\n\tvector.FillCircle(dst, 1, 2, 3, nil, true)\n}\n"})
		found, err := f.run(t, "# drawn once\npkg/draw/draw.go Painter.stamp  drawn once, at start-up\n")
		require.NoError(t, err)

		t.Run("it should report nothing", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})

	t.Run("when an allowlisted function no longer exists", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": source("")})
		found, err := f.run(t, "pkg/draw/draw.go Painter.gone  it was here once\n")
		require.NoError(t, err)

		t.Run("it should report the stale entry", func(t *testing.T) {
			assert.Equal(t, []string{"allowlist: pkg/draw/draw.go Painter.gone names no function"}, found)
		})
	})

	t.Run("when an allowlist entry gives no reason", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": source("")})
		_, err := f.run(t, "pkg/draw/draw.go draw\n")

		t.Run("it should refuse the allowlist", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the only anti-aliased call is in a _test.go file", func(t *testing.T) {
		f := newFixture(t, map[string]string{
			"pkg/draw/draw.go":      source(""),
			"pkg/draw/draw_test.go": source("\tvector.FillCircle(dst, 1, 2, 3, nil, true)"),
		})
		found, err := f.run(t, "")
		require.NoError(t, err)

		t.Run("it should report nothing", func(t *testing.T) {
			assert.Empty(t, found)
		})
	})

	t.Run("when the directory holds no Go files", func(t *testing.T) {
		f := newFixture(t, map[string]string{"README.md": "nothing here\n"})
		_, err := f.run(t, "")

		t.Run("it should fail, since every check passes on nothing", func(t *testing.T) {
			assert.Error(t, err)
		})
	})

	t.Run("when the vector package declares no anti-aliasing switch", func(t *testing.T) {
		f := newFixture(t, map[string]string{"pkg/draw/draw.go": source("")})
		require.NoError(t, os.WriteFile(filepath.Join(f.vector, "vector.go"), []byte("package vector\n\nfunc FillPath(dst any) {}\n"), 0o644))
		_, err := f.run(t, "")

		t.Run("it should fail, since it would look for nothing", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}
