package targets

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LineRange is a run of lines in a file, both ends included.
type LineRange struct{ From, To int }

func (r LineRange) overlaps(from, to int) bool { return r.From <= to && from <= r.To }

var hunk = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// ChangedLines reads git diff -U0 and gives each file the lines of its new
// version that each hunk covers. A hunk that only deletes covers the lines
// either side of it, so the declaration around a deletion still counts.
func ChangedLines(diff string) map[string][]LineRange {
	out := map[string][]LineRange{}
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		if name, ok := strings.CutPrefix(line, "+++ b/"); ok {
			file = name
			continue
		}
		m := hunk.FindStringSubmatch(line)
		if m == nil || file == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count == 0 {
			count = 2
		}
		out[file] = append(out[file], LineRange{From: start, To: start + count - 1})
	}
	return out
}

// node is a package-level declaration: a function, a method, or a name a
// var, const or type declaration binds.
type node struct {
	name     string
	fn       *ast.FuncDecl
	from, to int
	file     string
}

type graph struct {
	fset  *token.FileSet
	funcs map[string][]*node
	names map[string][]*node
	byFn  map[*ast.FuncDecl]*node
	all   []*node
}

func parsePackage(dir string) (*graph, error) {
	g := &graph{fset: token.NewFileSet(), funcs: map[string][]*node{}, names: map[string][]*node{}, byFn: map[*ast.FuncDecl]*node{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if ok, err := build.Default.MatchFile(dir, e.Name()); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(g.fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				n := g.add(e.Name(), d.Name.Name, d, d.Pos(), d.End())
				g.funcs[d.Name.Name] = append(g.funcs[d.Name.Name], n)
				g.byFn[d] = n
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, id := range s.Names {
							g.names[id.Name] = append(g.names[id.Name], g.add(e.Name(), id.Name, nil, s.Pos(), s.End()))
						}
					case *ast.TypeSpec:
						g.names[s.Name.Name] = append(g.names[s.Name.Name], g.add(e.Name(), s.Name.Name, nil, s.Pos(), s.End()))
					}
				}
			}
		}
	}
	return g, nil
}

func (g *graph) add(file, name string, fn *ast.FuncDecl, from, to token.Pos) *node {
	n := &node{name: name, fn: fn, from: g.fset.Position(from).Line, to: g.fset.Position(to).Line, file: file}
	g.all = append(g.all, n)
	return n
}

// reached is every declaration fn uses, through the package's own
// functions and methods: a bare name to the functions and names of that
// name, a selector to the methods of that name, whatever the receiver.
func (g *graph) reached(fn *node) map[*node]bool {
	seen := map[*node]bool{}
	var walk func(n *node)
	walk = func(n *node) {
		if seen[n] {
			return
		}
		seen[n] = true
		if n.fn == nil {
			return
		}
		var visit func(x ast.Node) bool
		visit = func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.SelectorExpr:
				for _, m := range g.funcs[x.Sel.Name] {
					if m.fn.Recv != nil {
						walk(m)
					}
				}
				ast.Inspect(x.X, visit)
				return false
			case *ast.Ident:
				for _, f := range g.funcs[x.Name] {
					if f.fn.Recv == nil {
						walk(f)
					}
				}
				for _, v := range g.names[x.Name] {
					walk(v)
				}
			}
			return true
		}
		ast.Inspect(n.fn.Body, visit)
	}
	walk(fn)
	return seen
}

// ReachingChange names the tests in the acceptance package at dir that reach
// a declaration whose lines in file changed. It cannot say, and why says
// why, for a file that is not Go or a change TestMain reaches, since every
// test runs through TestMain.
func ReachingChange(dir, file string, lines []LineRange) ([]string, string) {
	if !strings.HasSuffix(file, ".go") {
		return nil, file + " is not Go, so the call graph cannot say which tests it reaches"
	}
	g, err := parsePackage(dir)
	if err != nil {
		return nil, err.Error()
	}
	base := filepath.Base(file)
	changed := map[*node]bool{}
	for _, n := range g.all {
		if n.file != base {
			continue
		}
		for _, r := range lines {
			if r.overlaps(n.from, n.to) {
				changed[n] = true
			}
		}
	}
	if len(changed) == 0 {
		return nil, ""
	}
	for _, main := range g.funcs["TestMain"] {
		for n := range g.reached(main) {
			if changed[n] {
				return nil, fmt.Sprintf("TestMain reaches %s, which changed in %s", n.name, file)
			}
		}
	}
	var tests []string
	for name, ns := range g.funcs {
		for _, n := range ns {
			if n.fn.Recv != nil || !isTest(n.fn) {
				continue
			}
			for r := range g.reached(n) {
				if changed[r] {
					tests = append(tests, name)
					break
				}
			}
		}
	}
	sort.Strings(tests)
	return tests, ""
}
