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

// Tier is how an acceptance test runs (item 226). Performance tests
// measure CPU or memory, need a quiet machine, and run only when asked for;
// behaviour tests run on every landing, each in a process of its own.
type Tier int

const (
	Behaviour Tier = iota
	Performance
)

func (t Tier) String() string {
	return [...]string{"behaviour", "performance"}[t]
}

// Marker is the call a performance test makes among its own statements.
const Marker = "performanceTier"

// performanceNames are pkg/measure's CPU and instruction meters. The rest
// of it is the work counters, which are per process, so a test reading them
// is alone in its process by construction.
var performanceNames = map[string]bool{
	"Available": true, "Process": true, "ProcessCounter": true, "CountInstructions": true,
	"Thread": true, "Cost": true, "Total": true, "CoreRate": true,
}

// MetersFile holds every function of the package's own that reads CPU or
// memory. A /proc read elsewhere finds a process; one here measures it.
const MetersFile = "meters_test.go"

// Test is what ReadTiers learns about one top-level test: the tier its
// marker names, and the tier what it reaches calls for.
type Test struct {
	Tier    Tier
	Reaches Tier
}

// ReadTiers parses every Go file in an acceptance package. A test that
// calls performanceTier(t) among its own statements is in the performance
// tier, and any other in the behaviour tier. What it reaches, through the
// package's own functions and methods, decides the tier it needs: a function
// in MetersFile or one of pkg/measure's meters is Performance.
func ReadTiers(dir, measurePath string) (map[string]Test, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	funcs := map[string][]*ast.FuncDecl{}
	direct := map[*ast.FuncDecl]Tier{}
	var tests []*ast.FuncDecl
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if ok, err := build.Default.MatchFile(dir, e.Name()); err != nil || !ok {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		measureName := importName(f, measurePath)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			funcs[fn.Name.Name] = append(funcs[fn.Name.Name], fn)
			direct[fn] = needs(fn, measureName)
			if e.Name() == MetersFile {
				direct[fn] = Performance
			}
			if fn.Recv == nil && isTest(fn) {
				tests = append(tests, fn)
			}
		}
	}
	out := map[string]Test{}
	for _, fn := range tests {
		out[fn.Name.Name] = Test{Tier: tierOf(fn), Reaches: reach(fn, funcs, direct, map[*ast.FuncDecl]bool{})}
	}
	return out, nil
}

func importName(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == path {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return path[strings.LastIndex(path, "/")+1:]
		}
	}
	return ""
}

func isTest(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if !strings.HasPrefix(name, "Test") || name == "TestMain" || len(fn.Type.Params.List) != 1 {
		return false
	}
	star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "T"
}

func tierOf(fn *ast.FuncDecl) Tier {
	param := ""
	if names := fn.Type.Params.List[0].Names; len(names) == 1 {
		param = names[0].Name
	}
	for _, stmt := range fn.Body.List {
		expr, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			continue
		}
		f, ok := call.Fun.(*ast.Ident)
		if a, isIdent := call.Args[0].(*ast.Ident); ok && isIdent && f.Name == Marker && a.Name == param {
			return Performance
		}
	}
	return Behaviour
}

func needs(fn *ast.FuncDecl, measureName string) Tier {
	tier := Behaviour
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && measureName != "" && x.Name == measureName && performanceNames[sel.Sel.Name] {
				tier = Performance
			}
		}
		return tier != Performance
	})
	return tier
}

// InTier names the tests in one tier, sorted.
func InTier(tests map[string]Test, tier Tier) []string {
	var names []string
	for name, test := range tests {
		if test.Tier == tier {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// Matching narrows names to those pattern matches, all of them when it is
// empty, and refuses when it matches none, so a mistyped pattern cannot
// pass on nothing.
func Matching(names []string, pattern string) ([]string, error) {
	if pattern == "" {
		return names, nil
	}
	match, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range names {
		if match.MatchString(name) {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%q matches no test in the tier", pattern)
	}
	return out, nil
}

// RunPattern is the go test -run pattern that matches exactly these tests:
// none, when there are none.
func RunPattern(names []string) string {
	if len(names) == 0 {
		return "^$"
	}
	return "^(" + strings.Join(names, "|") + ")$"
}

// reach follows every name the function uses that the package declares: a
// bare name to its functions, and a selector to its methods of that name,
// whatever the receiver, so it over-counts rather than misses.
func reach(fn *ast.FuncDecl, funcs map[string][]*ast.FuncDecl, direct map[*ast.FuncDecl]Tier, seen map[*ast.FuncDecl]bool) Tier {
	if seen[fn] {
		return Behaviour
	}
	seen[fn] = true
	tier := direct[fn]
	follow := func(name string, methods bool) {
		for _, callee := range funcs[name] {
			if (callee.Recv != nil) == methods {
				tier = max(tier, reach(callee, funcs, direct, seen))
			}
		}
	}
	var visit func(n ast.Node) bool
	visit = func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			follow(n.Sel.Name, true)
			ast.Inspect(n.X, visit)
			return false
		case *ast.Ident:
			follow(n.Name, false)
		}
		return tier != Performance
	}
	ast.Inspect(fn.Body, visit)
	return tier
}
