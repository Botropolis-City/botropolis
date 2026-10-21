// Package check finds anti-aliased vector drawing in the module (item 191).
//
// An anti-aliased vector call in Ebitengine v2.10 draws its path several
// times through offscreen stencil images; per frame, across the city,
// that was most of the indices and most of the heap. The painter in
// pkg/shapes draws the same shapes as small meshes, and this keeps the
// expensive calls from creeping back. It reads which functions take an
// anti-aliasing switch from the vector package's own source, so a new or
// renamed one is covered, and resolves calls by import path, so renaming
// the import does not hide one.
package check

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Finding is one anti-aliased call, or one allowlist entry naming no
// function.
type Finding struct {
	File string
	Line int
	What string
}

func (f Finding) String() string {
	if f.Line == 0 {
		return "allowlist: " + f.What
	}
	return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.What)
}

type entry struct{ file, fn string }

// Run checks every non-test Go file under root against the vector
// package whose source is in vectorDir and whose import path is
// vectorPath, letting through only calls inside the functions allow
// names: one "file function reason" a line, the function as Name or
// Type.Name.
func Run(root, vectorDir, vectorPath string, allow io.Reader) ([]Finding, error) {
	switches, err := antialiasSwitches(vectorDir)
	if err != nil {
		return nil, err
	}
	allowed, err := readAllow(allow)
	if err != nil {
		return nil, err
	}
	files, err := goFiles(root)
	if err != nil {
		return nil, err
	}
	var found []Finding
	defined := map[entry]bool{}
	fset := token.NewFileSet()
	for _, rel := range files {
		file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, 0)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		names := importNames(file, vectorPath)
		for _, decl := range file.Decls {
			fn := funcName(decl)
			if fn != "" {
				defined[entry{rel, fn}] = true
			}
			if allowed[entry{rel, fn}] {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if what := offence(n, names, switches); what != "" {
					found = append(found, Finding{File: rel, Line: fset.Position(n.Pos()).Line, What: what})
				}
				return true
			})
		}
	}
	for e := range allowed {
		if !defined[e] {
			found = append(found, Finding{What: e.file + " " + e.fn + " names no function"})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		if found[i].Line != found[j].Line {
			return found[i].Line < found[j].Line
		}
		return found[i].What < found[j].What
	})
	return found, nil
}

// antialiasSwitches is every exported function of the vector package
// whose last parameter is an antialias bool.
func antialiasSwitches(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	switches := map[string]bool{}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			params := fn.Type.Params.List
			if len(params) == 0 {
				continue
			}
			last := params[len(params)-1]
			typ, ok := last.Type.(*ast.Ident)
			if !ok || typ.Name != "bool" || len(last.Names) == 0 {
				continue
			}
			if strings.Contains(strings.ToLower(last.Names[len(last.Names)-1].Name), "antialias") {
				switches[fn.Name.Name] = true
			}
		}
	}
	if len(switches) == 0 {
		return nil, fmt.Errorf("found no function with an antialias switch in %s, so there is nothing to look for", dir)
	}
	return switches, nil
}

func readAllow(r io.Reader) (map[entry]bool, error) {
	allowed := map[entry]bool{}
	scan := bufio.NewScanner(r)
	for n := 1; scan.Scan(); n++ {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("allowlist line %d: want a file, a function and the reason it may anti-alias: %q", n, line)
		}
		allowed[entry{fields[0], fields[1]}] = true
	}
	return allowed, scan.Err()
}

// goFiles is every non-test Go file under root, relative to it, outside
// hidden directories, testdata and nested modules.
func goFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root {
				if strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("found no Go files under " + root + ", and every check passes on nothing")
	}
	return files, nil
}

// importNames is the names a file refers to the vector package by; "."
// when it is dot-imported.
func importNames(file *ast.File, vectorPath string) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil || p != vectorPath {
			continue
		}
		switch {
		case spec.Name == nil:
			names[path.Base(p)] = true
		case spec.Name.Name != "_":
			names[spec.Name.Name] = true
		}
	}
	return names
}

func funcName(decl ast.Decl) string {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if index, ok := typ.(*ast.IndexExpr); ok {
		typ = index.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// offence says what is anti-aliased at n, or nothing. Only the literal
// false is known to switch anti-aliasing off.
func offence(n ast.Node, names, switches map[string]bool) string {
	switch n := n.(type) {
	case *ast.CallExpr:
		var called string
		switch fun := n.Fun.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok && names[pkg.Name] {
				called = fun.Sel.Name
			}
		case *ast.Ident:
			if names["."] {
				called = fun.Name
			}
		}
		if switches[called] && len(n.Args) > 0 && !isFalse(n.Args[len(n.Args)-1]) {
			return "vector." + called + " with anti-aliasing on"
		}
	case *ast.KeyValueExpr:
		if key, ok := n.Key.(*ast.Ident); ok && key.Name == "AntiAlias" && !isFalse(n.Value) {
			return "AntiAlias set on"
		}
	case *ast.AssignStmt:
		for i, lhs := range n.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "AntiAlias" && i < len(n.Rhs) && !isFalse(n.Rhs[i]) {
				return "AntiAlias set on"
			}
		}
	}
	return ""
}

func isFalse(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "false"
}
