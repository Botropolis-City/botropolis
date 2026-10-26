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
	also  []string
	reads map[string]string
}{
	kindDistrictFloor: {draw: "districtFloor", reads: agedFloorReads},
	kindLotFloor: {draw: "lotFloor", reads: with(agedFloorReads, map[string]string{
		"g.reduced":    "frameKey",
		"g.weedFade":   "worked out by dressing, from the lot's mowing and reduced motion, before it is read",
		"scene.Mowing": "the lot's key",
	})},
	kindHouse: {draw: "paintBuilding", reads: houseReads},
	kindMonument: {draw: "districtMonument", reads: because("g.rec", "g.paint", "g.kits", "g.drawing", "scene.View",
		"g.faces", "g.share", "g.jn", "g.textSeen", "g.glows",
		"g.baking", "g.paintUncut", "g.lightsOn", "g.litBounds", "g.litRects", "g.lightOps", "g.punches", "g.cuts")},
	kindSign: {draw: "paintDistrictSign", reads: because("g.rec", "g.paint", "g.theme", "g.faces", "g.share", "g.jn", "g.textSeen",
		"g.paintUncut", "g.lightsOn", "g.litBounds", "g.litRects", "g.lightOps", "g.cuts")},
	kindTree: {draw: "isoTree", reads: because("g.rec", "g.paint", "g.kits", "g.drawing", "scene.View",
		"g.baking", "g.lightsOn", "g.litBounds", "g.lightOps", "g.punches")},
	kindFog: {draw: "drawFog", reads: because("g.rec", "g.paint", "g.kits", "g.fogPuff")},
	kindBridge: {draw: "drawSpan", also: []string{"drawRampSegment", "drawHazePiece"}, reads: because("g.rec", "g.paint", "g.kits", "g.drawing", "scene.View",
		"g.baking", "g.lightsOn", "g.litBounds", "g.lightOps", "g.punches")},
}

// why is how a keyed draw's key covers each thing a draw can read.
var why = map[string]string{
	"g.rec":        "the recorder: where the draws go",
	"g.paint":      "the painter, which hands every draw to the recorder",
	"g.kits":       "frameKey: the kits' generation, and the zoom the level is picked by",
	"g.drawing":    "a sink: world draws, counted, and counted again when kept",
	"scene.View":   "frameKey: whether a view drains the colour",
	"g.theme":      "houseWide, in its key",
	"g.faces":      "houseWide, in its key",
	"g.share":      "houseWide, in its key: the share and whether it is yours",
	"g.jn":         "the journal: words, told again when kept",
	"g.textSeen":   "a sink: words, journalled and told again when kept",
	"g.glows":      "the journal: glows, laid again when kept",
	"g.baking":     "set only while a building's look bakes, which nothing else is drawn inside",
	"g.paintUncut": "set only while a building's look bakes, which nothing else is drawn inside",
	"g.lightsOn":   "frameKey: lights, and the panes lit before it",
	"g.litBounds":  "frameKey: the panes lit before it",
	"g.litRects":   "frameKey: the panes lit before it",
	"g.lightOps":   "the journal: light ops, laid again when kept",
	"g.punches":    "a sink: counted, and counted again when kept",
	"g.cuts":       "a sink: counted, and counted again when kept",
	"g.fogPuff":    "the puff texture, made once the first time fog is drawn and the same after",
}

// because is the reasons why gives for reads.
func because(reads ...string) map[string]string {
	out := map[string]string{}
	for _, r := range reads {
		reason, ok := why[r]
		if !ok {
			panic("no reason for " + r)
		}
		out[r] = reason
	}
	return out
}

// houseReads is what a building's painting reads beyond its frame, and
// how its key covers it.
var houseReads = map[string]string{
	"g.rec":         "the recorder: where the draws go",
	"g.paint":       "the painter, which hands every draw to the recorder",
	"g.kits":        "frameKey: the kits' generation, and the zoom the level is picked by",
	"g.drawing":     "a sink: world draws, counted, and counted again when kept",
	"g.reduced":     "frameKey",
	"scene.View":    "frameKey: whether a view drains the colour",
	"g.looks":       "frameKey: the cache; a kept look must still be cached as the same look",
	"g.lookPages":   "frameKey: the pages and the times one started again",
	"g.noLookCache": "frameKey",
	"g.lookStats":   "a sink: counted, and counted again when kept",
	"g.lookFrame":   "the frame a look is used at, noted again when kept; and where a look bakes",
	"g.frameBounds": "read where a look bakes, which keeps no record",
	"g.still":       "the scratch a look bakes through, which keeps no record",
	"g.jn":          "the journal: words and looks, done again when kept",
	"g.textSeen":    "a sink: words, journalled and told again when kept",
	"g.theme":       "houseWide",
	"g.faces":       "houseWide",
	"g.signage":     "houseWide",
	"g.share":       "houseWide: the share and whether it is yours",
	"g.lightsOn":    "frameKey: lights, and the panes lit before it",
	"g.litHash":     "frameKey: the panes lit before it; folded again when kept",
	"g.litRects":    "the journal: panes, laid again when kept",
	"g.litBounds":   "the journal: widened again when kept",
	"g.lightOps":    "the journal: light ops, laid again when kept",
	"g.meshCut":     "set afresh by every drawable, and by a kept one",
	"g.cuts":        "a sink: counted, and counted again when kept",
	"g.punches":     "a sink: counted, and counted again when kept",
	"g.windowDraws": "a sink: counted, and counted again when kept",
	"g.baking":      "set only while a look bakes, which keeps no record",
	"g.paintUncut":  "set only while a look bakes, which keeps no record",
	// The clock reaches the painting only through attention, which only
	// a pulsing beacon reads, and a pulsing beacon is a live step.
	"g.driven":            "the clock, read only by a pulsing beacon, a live step",
	"g.record":            "the clock, read only by a pulsing beacon, a live step",
	"g.ticks":             "the clock, read only by a pulsing beacon, a live step",
	"g.started":           "the clock, read only by a pulsing beacon, a live step",
	"g.motion":            "the clock, read only by a pulsing beacon, a live step",
	"g.liveliness":        "the clock, read only by a pulsing beacon, a live step",
	"scene.City":          "buildingKey: the title, from the city's time",
	"scene.Now":           "buildingKey: whether the top course is lit",
	"scene.Lights":        "its frame: lit as its rise, its construction and its haul are, all in its frame",
	"scene.Hoist":         "buildingKey: its flags' hoists",
	"scene.Scaffold":      "buildingKey: its scaffold",
	"scene.Rising":        "its frame's rise",
	"scene.Celebration":   "its frame's state: a celebration is a live step",
	"scene.CompactionJob": "its frame's state: a course pouring is a live step",
	"scene.Loads":         "its frame's state: loads on the hoist are a live step",
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
func gameReads(t *testing.T, root string) map[string]string {
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
	reads, seen := map[string]string{}, map[*ast.FuncDecl]bool{}
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
					if _, ok := reads["g."+n.Sel.Name]; !ok {
						reads["g."+n.Sel.Name] = fn.Name.Name
					}
					return true
				}
				if inner, ok := n.X.(*ast.SelectorExpr); ok {
					if id, ok := inner.X.(*ast.Ident); ok && id.Name == "g" && inner.Sel.Name == "scene" {
						if _, ok := reads["scene."+n.Sel.Name]; !ok {
							reads["scene."+n.Sel.Name] = fn.Name.Name
						}
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
			for _, more := range d.also {
				for r, in := range gameReads(t, more) {
					if _, ok := reads[r]; !ok {
						reads[r] = in
					}
				}
			}
			var unkeyed, unread []string
			for r, in := range reads {
				if _, ok := d.reads[r]; !ok {
					unkeyed = append(unkeyed, r+" (in "+in+")")
				}
			}
			for r := range d.reads {
				if _, ok := reads[r]; !ok {
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
