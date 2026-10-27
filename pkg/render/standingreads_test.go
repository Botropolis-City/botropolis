package render

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// source is the package's code, parsed once: its files, and its
// functions and the game's methods by name.
type source struct {
	fset    *token.FileSet
	files   []*ast.File
	methods map[string]*ast.FuncDecl
	funcs   map[string]*ast.FuncDecl
}

func parseSource(t *testing.T) *source {
	t.Helper()
	names, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	s := &source{fset: fset, methods: map[string]*ast.FuncDecl{}, funcs: map[string]*ast.FuncDecl{}}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		s.files = append(s.files, f)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Recv == nil {
				s.funcs[fn.Name.Name] = fn
				continue
			}
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "Game" {
					s.methods[fn.Name.Name] = fn
				}
			}
		}
	}
	return s
}

// keyedSite is a call that keeps a record: its kind, as written, and
// the code around it that works out its key and its draw, the loop body
// it is in or else the function.
type keyedSite struct {
	kind  string
	in    string
	scope ast.Node
	// passed is what the callers of the function it is in hand it,
	// where its key or draw takes a parameter: the arguments they pass,
	// and what those are worked out from.
	passed []ast.Node
}

// keyedSites is every call of noteKeyed in the package's code, so a kind
// is walked the moment one draws it, with nothing to remember.
func (s *source) keyedSites() []keyedSite {
	var sites []keyedSite
	for _, f := range s.files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var stack []ast.Node
			ast.Inspect(fn, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "noteKeyed" || len(call.Args) != 3 {
					return true
				}
				kind, _ := call.Args[0].(*ast.Ident)
				site := keyedSite{in: fn.Name.Name, scope: fn}
				if kind != nil {
					site.kind = kind.Name
				}
				for i := len(stack) - 1; i >= 0; i-- {
					switch loop := stack[i].(type) {
					case *ast.ForStmt:
						site.scope = loop.Body
					case *ast.RangeStmt:
						site.scope = loop.Body
					case *ast.IfStmt:
						site.scope = loop.Body
					case *ast.FuncLit:
						if len(loop.Body.List) < 2 {
							continue
						}
						site.scope = loop.Body
						// A closure passed to a call, with its draw from a
						// sibling closure: the whole call.
						if i > 0 {
							if call, ok := stack[i-1].(*ast.CallExpr); ok {
								site.scope = call
							}
						}
					default:
						continue
					}
					break
				}
				site.passed = s.passedTo(fn, call.Args, map[string]bool{})
				sites = append(sites, site)
				return true
			})
		}
	}
	return sites
}

// passedTo is, where args use a parameter of fn, every argument fn's
// callers hand it and what each is worked out from in the caller, and so
// on up through callers whose own parameters those use (item 212).
func (s *source) passedTo(fn *ast.FuncDecl, args []ast.Expr, seen map[string]bool) []ast.Node {
	// drawIso is where a frame's still city starts: what it is handed,
	// the scene's city, the camera, the hover, the selection and the
	// clock, the standing key covers by name.
	if seen[fn.Name.Name] || fn.Name.Name == "drawIso" || !usesParams(fn, args) {
		return nil
	}
	seen[fn.Name.Name] = true
	var out []ast.Node
	for _, f := range s.files {
		for _, d := range f.Decls {
			caller, ok := d.(*ast.FuncDecl)
			if !ok || caller.Body == nil {
				continue
			}
			var path []ast.Node
			ast.Inspect(caller.Body, func(n ast.Node) bool {
				if n == nil {
					path = path[:len(path)-1]
					return true
				}
				path = append(path, n)
				call, ok := n.(*ast.CallExpr)
				if !ok || !calls(call, fn) {
					return true
				}
				for _, a := range call.Args {
					out = append(out, a)
				}
				out = append(out, definitions(caller, call, path)...)
				out = append(out, s.passedTo(caller, call.Args, seen)...)
				return true
			})
		}
	}
	return out
}

// calls is whether call calls fn: a method of the game, or a function.
func calls(call *ast.CallExpr, fn *ast.FuncDecl) bool {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		id, ok := f.X.(*ast.Ident)
		return fn.Recv != nil && ok && id.Name == "g" && f.Sel.Name == fn.Name.Name
	case *ast.Ident:
		return fn.Recv == nil && f.Name == fn.Name.Name
	}
	return false
}

// names is every identifier in nodes.
func names(nodes ...ast.Node) map[string]bool {
	out := map[string]bool{}
	for _, n := range nodes {
		ast.Inspect(n, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				out[id.Name] = true
			}
			return true
		})
	}
	return out
}

// usesParams is whether args name a parameter of fn that carries what
// something is drawn from. The image drawn on and the camera do not:
// the camera is in the standing key whole, by the frame's base and its
// origin.
func usesParams(fn *ast.FuncDecl, args []ast.Expr) bool {
	used := names(exprs(args)...)
	for _, f := range fn.Type.Params.List {
		if drawnWith(f.Type) {
			continue
		}
		for _, name := range f.Names {
			if used[name.Name] {
				return true
			}
		}
	}
	return false
}

// drawnWith is whether a parameter's type is what a draw is made with
// rather than from: the image drawn on, or the camera.
func drawnWith(t ast.Expr) bool {
	star, ok := t.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "Image" || sel.Sel.Name == "Camera")
}

func exprs(es []ast.Expr) []ast.Node {
	out := make([]ast.Node, len(es))
	for i, e := range es {
		out[i] = e
	}
	return out
}

// definitions is what the names call's arguments use are worked out
// from, where they are given in the blocks around call (path is the
// nodes from fn's body down to it): the right of each assignment or
// declaration before it in one of those blocks, or of a range or an if's
// statement it is inside, and of those that give the names they use in
// turn. A name given elsewhere in fn is another thing of that name.
func definitions(fn *ast.FuncDecl, call *ast.CallExpr, path []ast.Node) []ast.Node {
	want := names(exprs(call.Args)...)
	for _, f := range fn.Type.Params.List {
		if drawnWith(f.Type) {
			for _, name := range f.Names {
				delete(want, name.Name)
			}
		}
	}
	delete(want, "screen")
	// givers is every statement around call that can give a name.
	var givers []ast.Node
	for k, n := range path {
		switch n := n.(type) {
		case *ast.BlockStmt:
			for _, st := range n.List {
				if st.Pos() < call.Pos() && (k+1 >= len(path) || st != path[k+1]) {
					givers = append(givers, st)
				}
			}
		case *ast.RangeStmt:
			givers = append(givers, n)
		case *ast.IfStmt:
			if n.Init != nil {
				givers = append(givers, n.Init)
			}
		case *ast.ForStmt:
			if n.Init != nil {
				givers = append(givers, n.Init)
			}
		}
	}
	done := map[ast.Node]bool{}
	var out []ast.Node
	for grew := true; grew; {
		grew = false
		for _, g := range givers {
			if done[g] {
				continue
			}
			var lhs, rhs []ast.Expr
			switch g := g.(type) {
			case *ast.AssignStmt:
				lhs, rhs = g.Lhs, g.Rhs
			case *ast.RangeStmt:
				lhs, rhs = []ast.Expr{g.Key, g.Value}, []ast.Expr{g.X}
			case *ast.DeclStmt:
				if gen, ok := g.Decl.(*ast.GenDecl); ok {
					for _, spec := range gen.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range v.Names {
								lhs = append(lhs, name)
							}
							rhs = append(rhs, v.Values...)
						}
					}
				}
			}
			for _, l := range lhs {
				if id, ok := l.(*ast.Ident); ok && want[id.Name] {
					done[g], grew = true, true
					for _, r := range rhs {
						out = append(out, r)
						for name := range names(r) {
							if name != "screen" && name != "cam" {
								want[name] = true
							}
						}
					}
					break
				}
			}
		}
	}
	return out
}

// reads is everything of the game and the scene the code from node reads,
// through every method of the game and function of the package it
// calls, each with the function it was first read in, and the functions
// that read the clock. It does not go into noteKeyed, which keeps or
// records what it is given.
func (s *source) reads(node ast.Node, in string, passed ...ast.Node) (reads, clocks map[string]string) {
	reads, clocks = map[string]string{}, map[string]string{}
	seen := map[*ast.FuncDecl]bool{}
	note := func(read, in string) {
		if _, ok := reads[read]; !ok {
			reads[read] = in
		}
	}
	var walk func(n ast.Node, in string, params *ast.FieldList)
	walk = func(n ast.Node, in string, params *ast.FieldList) {
		// Names the scene and the city go by in this function: from
		// g.scene and g.scene.City(), and parameters of their types.
		scenes, cities := map[string]bool{}, map[string]bool{}
		if params != nil {
			for _, f := range params.List {
				star, ok := f.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				sel, ok := star.X.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				for _, name := range f.Names {
					switch sel.Sel.Name {
					case "Scene":
						scenes[name.Name] = true
					case "City":
						cities[name.Name] = true
					}
				}
			}
		}
		ast.Inspect(n, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
				for i, rhs := range as.Rhs {
					id, ok := as.Lhs[i].(*ast.Ident)
					if !ok {
						continue
					}
					if sel, ok := rhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "scene" {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == "g" {
							scenes[id.Name] = true
						}
					}
					if call, ok := rhs.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "City" {
							cities[id.Name] = true
						}
					}
				}
			}
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					switch {
					case scenes[id.Name]:
						note("scene."+sel.Sel.Name, in)
						if sel.Sel.Name == "Now" {
							clocks[in] = "scene.Now"
						}
						return false
					case cities[id.Name]:
						note("city."+sel.Sel.Name, in)
						return false
					}
				}
				if call, ok := sel.X.(*ast.CallExpr); ok {
					if inner, ok := call.Fun.(*ast.SelectorExpr); ok && inner.Sel.Name == "City" {
						note("city."+sel.Sel.Name, in)
					}
				}
			}
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "g" {
					if m, ok := s.methods[n.Sel.Name]; ok {
						if n.Sel.Name == "now" {
							clocks[in] = "g.now"
						}
						if n.Sel.Name != "noteKeyed" && !seen[m] {
							seen[m] = true
							walk(m.Body, m.Name.Name, m.Type.Params)
						}
						return true
					}
					if _, ok := reads["g."+n.Sel.Name]; !ok {
						reads["g."+n.Sel.Name] = in
					}
					return true
				}
				if inner, ok := n.X.(*ast.SelectorExpr); ok {
					if id, ok := inner.X.(*ast.Ident); ok && id.Name == "g" && inner.Sel.Name == "scene" {
						if _, ok := reads["scene."+n.Sel.Name]; !ok {
							reads["scene."+n.Sel.Name] = in
						}
						if n.Sel.Name == "Now" {
							clocks[in] = "scene.Now"
						}
						return false
					}
				}
				if n.Sel.Name == "Now" {
					if id, ok := n.X.(*ast.Ident); ok && id.Name == "time" {
						clocks[in] = "time.Now"
					}
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok {
					if f, ok := s.funcs[id.Name]; ok && !seen[f] {
						seen[f] = true
						walk(f.Body, f.Name.Name, f.Type.Params)
					}
				}
			}
			return true
		})
	}
	walk(node, in, nil)
	for _, n := range passed {
		walk(n, in+" (passed)", nil)
	}
	delete(reads, "g.scene")
	return reads, clocks
}

// standingReasons is what a still drawable's key or draw reads that the
// standing key need not, each with why (item 212).
var standingReasons = map[string]string{
	"g.paint":           "the record being made",
	"g.paintUncut":      "the record being made",
	"g.drawing":         "the record being made",
	"g.still":           "the record kept, and the scratch it is made with",
	"g.noStill":         "whether the frame is drawn straight, which keeps nothing",
	"g.mu":              "a lock",
	"g.frameHits":       "a sink: hits, noted every frame",
	"g.mapLabels":       "a sink: labels, queued every frame",
	"g.windowDraws":     "a sink: counted",
	"g.landmarkDraws":   "a sink: counted",
	"g.buildingsDrawn":  "a sink: counted",
	"g.lookStats":       "a sink: counted",
	"g.textSeen":        "a sink: the words a frame drew",
	"g.jn":              "a sink: the journal of what a record drew",
	"g.glows":           "a sink: glows, journalled and replayed",
	"g.lightOps":        "a sink: light ops, journalled and replayed",
	"g.litBounds":       "a sink: where the lights fell",
	"g.litRects":        "a sink: lit panes, journalled",
	"g.litHash":         "the panes lit before, folded again as records are kept",
	"g.punches":         "a sink: the lights' punches",
	"g.cuts":            "a sink: the cut words, journalled",
	"g.meshCut":         "set afresh by every drawable",
	"g.baking":          "set while a look bakes, which unsettles the record",
	"g.lookFrame":       "a look's bake scratch",
	"g.frameBounds":     "a look's bake scratch",
	"g.quietFrames":     "a memo of quiet buildings' frames, by the quiet key",
	"g.lotKeys":         "a memo of the city: its buildings' values, reset with each snapshot",
	"g.titles":          "a memo of the city, reset with each snapshot",
	"g.signValues":      "a memo of the city, reset with each snapshot",
	"g.treeHashes":      "a memo of the city, reset with each snapshot",
	"g.isleHash":        "a memo of the city, reset with each snapshot",
	"g.bridgeHashes":    "a memo of the city, reset with each snapshot",
	"g.fogHashes":       "a memo of the city, reset with each snapshot",
	"g.landmarksKey":    "a memo of the city, reset with each snapshot",
	"g.landmarksHashed": "a memo of the city, reset with each snapshot",
	"g.dresser":         "a memo of the lots' dressing, by its generation",
	"g.fogPuff":         "the puff texture, made once",
	"g.weedFade":        "worked out by dressing, from mowing and reduced motion, before it is read",
	"g.linePairs":       "scratch: a buffer of segments",
	"g.theme":           "house-wide: worked out once a frame into g.houseWide, which the standing key folds",
	"g.faces":           "house-wide: worked out once a frame into g.houseWide, which the standing key folds",
	"g.signage":         "house-wide: worked out once a frame into g.houseWide, which the standing key folds",
	"g.share":           "house-wide: worked out once a frame into g.houseWide, which the standing key folds",
	"g.liveliness":      "how much the city moves, read for things that say they move: never kept",
	"g.motion":          "the clock moving things are drawn to: never kept",
	"g.started":         "the clock moving things are drawn to: never kept",
	"g.ticks":           "the clock moving things are drawn to: never kept",
	"g.record":          "the clock moving things are drawn to: never kept",
	"g.driven":          "the clock moving things are drawn to: never kept",

	"city.Bridges":     "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.FogOf":       "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Hall":        "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Clinic":      "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Library":     "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Plant":       "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.DistrictOf":  "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Driveway":    "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.InBlock":     "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.StreetCells": "the city, rebuilt with each snapshot, which scene.Generation counts",
	"city.Time":        "the snapshot's time, which scene.Generation counts",

	"scene.Celebration":   "stirring: celebrating, which the standing key folds",
	"scene.CompactionJob": "stirring: pouring, which the standing key folds",
	"scene.Hauling":       "stirring: hauled, which the standing key folds",
	"scene.HaulProgress":  "stirring: hauled, which the standing key folds",
	"scene.Loads":         "stirring: loads, which the standing key folds",
	"scene.Settling":      "stirring: settling, which the standing key folds",
}

// standingClocks is each function a still drawable reads the clock in,
// and how the standing key covers what it reads of it (item 212).
var standingClocks = map[string]string{
	"quietKey":    "a course's glow: the standing key folds courseLit for every building",
	"buildingKey": "a course's glow: the standing key folds courseLit for every building",
	"keyOf":       "a course's glow: the standing key folds courseLit for every building",
	"drawCourses": "a course's glow: the standing key folds courseLit for every building",
	"stepMotion":  "the time moving things are drawn to: a draw that reads it says it moves, and is never kept",
}

// TestStillKeyReads holds every still drawable's key and draw to the
// standing key: whatever they read, the standing key's own code reads
// too, or it is listed with why, so a frame that reuses the still city
// whole cannot hold a still thing whose drawing would have changed.
// The drawables are every noteKeyed call in the package's code, so a
// new kind is held to it the moment it is drawn.
func TestStillKeyReads(t *testing.T) {
	s := parseSource(t)
	key, ok := s.methods["standingKey"]
	require.True(t, ok, "precondition: the standing key is a method of the game")
	covered, coveredClocks := s.reads(key.Body, key.Name.Name)
	sites := s.keyedSites()

	t.Run("when the package's code keeps records", func(t *testing.T) {
		noted := map[string]bool{}
		for _, site := range sites {
			noted[site.kind] = true
		}
		t.Run("it should note every kind it keeps", func(t *testing.T) {
			assert.ElementsMatch(t, s.keptKinds(), slices.Collect(maps.Keys(noted)))
		})
	})

	used := map[string]bool{}
	for _, site := range sites {
		reads, clocks := s.reads(site.scope, site.in, site.passed...)
		for r := range reads {
			if _, ok := covered[r]; !ok {
				used[r] = true
			}
		}
		t.Run("when "+site.kind+"'s key or draw, in "+site.in+", reads the game's or the scene's state", func(t *testing.T) {
			var uncovered []string
			for r, in := range reads {
				if _, ok := covered[r]; ok {
					continue
				}
				if _, ok := standingReasons[r]; ok {
					continue
				}
				uncovered = append(uncovered, r+" (in "+in+")")
			}
			sort.Strings(uncovered)

			t.Run("it should be read by the standing key, or listed with a reason", func(t *testing.T) {
				assert.Empty(t, uncovered)
			})
		})
		t.Run("when "+site.kind+"'s key or draw, in "+site.in+", reads the clock", func(t *testing.T) {
			var unread []string
			for in, clock := range clocks {
				_, covered := coveredClocks[in]
				if _, ok := standingClocks[in]; !ok && !covered {
					unread = append(unread, in+" ("+clock+")")
				}
			}
			sort.Strings(unread)

			t.Run("it should be read there by the standing key, or listed with how it covers it", func(t *testing.T) {
				assert.Empty(t, unread)
			})
		})
	}

	t.Run("when a keyed draw gives back what the frame goes on to use", func(t *testing.T) {
		unsaid := s.givesBack()

		t.Run("it should say it runs every frame", func(t *testing.T) {
			assert.Empty(t, unsaid)
		})
	})

	t.Run("when a read is listed with a reason", func(t *testing.T) {
		var stale []string
		for r := range standingReasons {
			if !used[r] {
				stale = append(stale, r)
			}
		}
		sort.Strings(stale)

		t.Run("it should be one a still drawable makes", func(t *testing.T) {
			assert.Empty(t, stale)
		})
	})
}

// keptKinds is the kinds of record the package keeps, as written: the
// constants declared from kindDistrictFloor up to kindsKept.
func (s *source) keptKinds() []string {
	for _, f := range s.files {
		for _, d := range f.Decls {
			gen, ok := d.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			var names []string
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					names = append(names, name.Name)
				}
			}
			from, to := slices.Index(names, "kindDistrictFloor"), slices.Index(names, "kindsKept")
			if from >= 0 && to > from {
				return names[from:to]
			}
		}
	}
	return nil
}

// givesBack is every keyed draw, as an item sets it, that assigns
// something besides what it notes, where the block setting it does not
// also say the item runs: a frame holding the still city would skip
// the assignment.
func (s *source) givesBack() []string {
	var unsaid []string
	for _, f := range s.files {
		ast.Inspect(f, func(n ast.Node) bool {
			block, ok := n.(*ast.BlockStmt)
			if !ok {
				return true
			}
			runs := false
			for _, st := range block.List {
				as, ok := st.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 1 {
					continue
				}
				if sel, ok := as.Lhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "runs" {
					runs = true
				}
			}
			for _, st := range block.List {
				as, ok := st.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
					continue
				}
				sel, ok := as.Lhs[0].(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "keyed" {
					continue
				}
				lit, ok := as.Rhs[0].(*ast.FuncLit)
				if !ok {
					continue
				}
				assigns := false
				ast.Inspect(lit.Body, func(n ast.Node) bool {
					if _, ok := n.(*ast.FuncLit); ok {
						return false
					}
					if _, ok := n.(*ast.AssignStmt); ok {
						assigns = true
					}
					return true
				})
				if assigns && !runs {
					unsaid = append(unsaid, fmt.Sprint(s.fset.Position(as.Pos())))
				}
			}
			return true
		})
	}
	return unsaid
}
