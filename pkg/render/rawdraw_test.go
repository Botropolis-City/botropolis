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

// drawsAllowed is every function that may draw onto an image without
// going through the dry pass's recorder, and why. A draw the recorder
// does not see is not in the standing city's record: if it is part of
// the world, a still drawable that makes it goes stale and a mover that
// makes it leaves no rectangle behind it (item 193).
var drawsAllowed = map[string]string{
	// The recorder itself, drawing when the image is not the dry one,
	// and playing what it recorded.
	"ink.go:Image":       "the recorder",
	"ink.go:colorm":      "the recorder",
	"ink.go:Mesh":        "the recorder",
	"ink.go:triangles16": "the recorder",
	"ink.go:words":       "the recorder",
	"ink.go:fill":        "the recorder",
	"ink.go:playCmd":     "the recorder, playing",
	// The painter, which hands every draw onto its destination to its
	// Recorder when it has one.
	"painter.go:drawImage":    "hands the draw to the recorder when set",
	"painter.go:flush":        "hands the draw to the recorder when set",
	"painter.go:FillPath":     "the recorder's own way to fill a ring",
	"painter.go:CutTriangles": "cuts the night's lights image, laid from the record",
	"painter.go:CutPath":      "cuts the night's lights image, laid from the record",
	"painter.go:whitePixel":   "makes the painter's own texture",
	"painter.go:disc":         "makes the painter's own stamp",
	// Laying what was recorded: the ground, the standing city, its
	// lights and the fade.
	"view.go:blitBase":              "lays the composed ground",
	"stillframe.go:paintStill":      "lays the standing city",
	"stillframe.go:paintLights":     "lays the night's lights",
	"stillframe.go:paintOverlay":    "lays the group lines",
	"stillframe.go:layLightsStream": "lays light ops, recorded",
	"stillframe.go:shiftLayer":      "moves a layer composed from the record by a whole-pixel pan",
	"fadeplay.go:done":              "lays the fade layer",
	"driveways.go:newFader":         "lays the fade layer of a frame drawn straight",
	"windows.go:endLights":          "lays the night's lights of a frame drawn straight",
	"windows.go:layLights":          "lays light ops, recorded",
	"windows.go:layCuts":            "lays light ops, recorded",
	"lightcuts.go:layCut":           "lays light ops, recorded",
	"iso.go:wash":                   "the night's wash, drawn over the frame after the city",
	// Drawn over the city after it, never part of it.
	"maplabels.go:drawMapLabels":           "labels, laid over the world each frame",
	"cityplan.go:drawCityPlan":             "the plan, drawn over the city",
	"reader.go:runs":                       "a panel's text",
	"sheetlandmarks.go:sheetLandmark":      "the building sheet",
	"sheetlandmarks.go:sheetLandmarkByDay": "the building sheet",
	"game.go:Draw":                         "clears the frame before the kits are loaded",
	"syncdialog.go:drawSync":               "a panel, drawn instead of the city",
	"ground.go:ground":                     "the top-down map, which draws no standing city",
	"framecheck.go:worldFrame":             "the check of the standing city",
	"lookcheck.go:lookFrameOf":             "item 192's check",
	"lookcheck_night.go:nightFrame":        "item 192's check",
	"lookcheck_night.go:alone":             "item 192's check",
	"sheet.go:sheetScene":                  "the building sheet",
	"sheet.go:sheetRising":                 "the building sheet",
	"sheet.go:sheetNight":                  "the building sheet",
	"sheetfront.go:frontScene":             "the building sheet",
}

// imageDraws are ebiten.Image's own methods that put pixels on it, and
// packageDraws the functions of colorm, text and vector that do.
var (
	imageDraws = map[string]bool{
		"DrawImage": true, "DrawTriangles": true, "DrawTriangles32": true,
		"DrawTrianglesShader": true, "DrawTrianglesShader32": true, "DrawRectShader": true,
	}
	packageDraws = map[string]bool{
		"Draw": true, "DrawImage": true, "DrawTriangles": true, "DrawTriangles32": true,
		"FillPath": true, "StrokePath": true, "FillRect": true, "StrokeRect": true,
		"StrokeLine": true, "FillCircle": true, "StrokeCircle": true,
	}
	drawPackages = map[string]bool{"colorm": true, "text": true, "vector": true}
	// imageNames are the names an image goes by where a bare Fill on it
	// draws: the painter's and a mesh's Fill take points, not a colour.
	imageNames = map[string]bool{"screen": true, "dst": true, "img": true, "target": true}
)

// isRawDraw is whether a call puts pixels on an image itself.
func isRawDraw(sel *ast.SelectorExpr) bool {
	name := sel.Sel.Name
	recv, _ := sel.X.(*ast.Ident)
	switch {
	case recv != nil && drawPackages[recv.Name]:
		return packageDraws[name]
	case recv != nil && imageNames[recv.Name] && name == "Fill":
		return true
	}
	return imageDraws[name]
}

// rawDrawSites is every function in dir's non-test sources that draws
// onto an image directly, as "file:function".
func rawDrawSites(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	sites := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !isRawDraw(sel) {
					return true
				}
				sites[filepath.Base(path)+":"+fn.Name.Name] = true
				return true
			})
		}
	}
	return sites
}

// Item 193: every draw of the world goes through the dry pass's
// recorder, so the standing city's record holds all of it.
func TestRawDrawSites(t *testing.T) {
	sites := rawDrawSites(t, ".")
	for site := range rawDrawSites(t, filepath.Join("..", "shapes")) {
		sites[site] = true
	}

	t.Run("when pkg/render and pkg/shapes are read", func(t *testing.T) {
		var stray []string
		for site := range sites {
			if _, ok := drawsAllowed[site]; !ok {
				stray = append(stray, site)
			}
		}
		sort.Strings(stray)

		t.Run("it should find every draw onto an image in the recorder or a function allowed to draw", func(t *testing.T) {
			assert.Empty(t, stray)
		})
	})

	t.Run("when the functions allowed to draw are listed", func(t *testing.T) {
		var gone []string
		for site := range drawsAllowed {
			if !sites[site] {
				gone = append(gone, site)
			}
		}
		sort.Strings(gone)

		t.Run("it should list only functions that still draw", func(t *testing.T) {
			assert.Empty(t, gone)
		})
	})
}
