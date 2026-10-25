package render

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyedDraws is each kind of drawable that keeps its record, the method
// of the game that draws it, and everything that draw reads of the game
// and the scene, with how its key covers it (item 212). A read the walk
// finds and this does not list fails TestKeyedDrawsRead, so a draw that
// starts reading something new cannot keep a record its key does not
// cover.
var keyedDraws = map[recordKind]struct {
	draw  string
	reads map[string]string
}{
	kindDistrictFloor: {"districtFloor", agedFloorReads},
	kindLotFloor: {"lotFloor", with(agedFloorReads, map[string]string{
		"g.reduced":    "frameKey",
		"g.weedFade":   "worked out by dressing, from the lot's mowing and reduced motion, before it is read",
		"scene.Mowing": "the lot's key",
	})},
}

// agedFloorReads is what an aged floor reads, and how its key covers it.
var agedFloorReads = map[string]string{
	"g.rec":       "the recorder: where the draws go",
	"g.paint":     "the painter, which hands every draw to the recorder",
	"g.linePairs": "a buffer, emptied before each use",
	"g.drawing":   "a sink: world draws, counted, and counted again when kept",
	"g.kits":      "frameKey: the kits' generation, and the zoom the level is picked by",
	"scene.View":  "frameKey: whether a view drains the colour",
	"g.lightsOn":  "whether a punch is noted, and a light op noted keeps no record",
	"g.litBounds": "whether a punch is noted, and a light op noted keeps no record",
	"g.baking":    "set only while a building's look bakes, which no aged floor is drawn inside",
	"g.lightOps":  "a sink: a light op noted keeps no record",
	"g.punches":   "a sink: a punch noted keeps no record",
}

// with is reads and more, in one list.
func with(reads, more map[string]string) map[string]string {
	out := maps.Clone(reads)
	maps.Copy(out, more)
	return out
}

// gameReads walks every function of the package a method of the game
// can reach and returns what it reads of the game, as "g.field", and
// what it asks of the scene, as "scene.Method".
func gameReads(t *testing.T, root string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	methods, funcs := map[string]*ast.FuncDecl{}, map[string]*ast.FuncDecl{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Recv == nil {
				funcs[fn.Name.Name] = fn
				continue
			}
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "Game" {
					methods[fn.Name.Name] = fn
				}
			}
		}
	}
	require.Contains(t, methods, root, "precondition: the draw is a method of the game")
	reads, seen := map[string]bool{}, map[*ast.FuncDecl]bool{}
	var walk func(fn *ast.FuncDecl)
	walk = func(fn *ast.FuncDecl) {
		if seen[fn] {
			return
		}
		seen[fn] = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "g" {
					if m, ok := methods[n.Sel.Name]; ok {
						walk(m)
						return true
					}
					reads["g."+n.Sel.Name] = true
					return true
				}
				if inner, ok := n.X.(*ast.SelectorExpr); ok {
					if id, ok := inner.X.(*ast.Ident); ok && id.Name == "g" && inner.Sel.Name == "scene" {
						reads["scene."+n.Sel.Name] = true
						return false
					}
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok {
					if f, ok := funcs[id.Name]; ok {
						walk(f)
					}
				}
			}
			return true
		})
	}
	walk(methods[root])
	delete(reads, "g.scene")
	return reads
}

func TestKeyedDrawsRead(t *testing.T) {
	for kind, d := range keyedDraws {
		t.Run("when "+kind.String()+"'s draw reads the game's or the scene's state", func(t *testing.T) {
			reads := gameReads(t, d.draw)
			var unkeyed, unread []string
			for r := range reads {
				if _, ok := d.reads[r]; !ok {
					unkeyed = append(unkeyed, r)
				}
			}
			for r := range d.reads {
				if !reads[r] {
					unread = append(unread, r)
				}
			}
			sort.Strings(unkeyed)
			sort.Strings(unread)

			t.Run("it should be in its key, or listed with a reason", func(t *testing.T) {
				assert.Empty(t, unkeyed)
			})
			t.Run("it should list nothing it does not read", func(t *testing.T) {
				assert.Empty(t, unread)
			})
		})
	}
}
