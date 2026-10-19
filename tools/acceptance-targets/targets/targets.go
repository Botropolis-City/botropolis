// Package targets chooses which acceptance tests a change can reach, from a
// map of the packages each test executed (item 117).
package targets

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Map struct {
	Rev   string              `json:"rev"`
	Tests map[string][]string `json:"tests"`
}

type Choice struct {
	Tests    []string
	Fallback string
	Note     string
}

// PackageOf names the package a changed file belongs to: the nearest
// directory at or above it, under pkg/ or cmd/, that holds Go files. A
// deleted file still maps through the directory it was in.
func PackageOf(root, module, file string) (string, bool) {
	file = filepath.ToSlash(filepath.Clean(file))
	top, _, _ := strings.Cut(file, "/")
	if top != "pkg" && top != "cmd" {
		return "", false
	}
	for dir := path.Dir(file); dir != "." && dir != top; dir = path.Dir(dir) {
		if strings.HasSuffix(file, ".go") && dir == path.Dir(file) {
			return module + "/" + dir, true
		}
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				return module + "/" + dir, true
			}
		}
	}
	return "", false
}

func Choose(m *Map, tests, changed []string, pkgOf func(string) (string, bool)) Choice {
	if m == nil {
		return Choice{Fallback: "there is no map yet"}
	}
	if added, removed := diff(tests, m.Tests); len(added)+len(removed) > 0 {
		return Choice{Fallback: fmt.Sprintf("the tests changed since the map: added %v, removed %v", added, removed)}
	}
	if len(changed) == 0 {
		return Choice{Note: "nothing changed since the map was made at " + m.Rev}
	}
	pkgs := map[string]bool{}
	for _, f := range changed {
		pkg, ok := pkgOf(f)
		if !ok {
			return Choice{Fallback: f + " is outside pkg/ and cmd/, so the map cannot say which tests it reaches"}
		}
		pkgs[pkg] = true
	}
	var chosen []string
	for _, test := range tests {
		touched := m.Tests[test]
		if len(touched) == 0 {
			chosen = append(chosen, test)
			continue
		}
		for _, p := range touched {
			if pkgs[p] {
				chosen = append(chosen, test)
				break
			}
		}
	}
	sort.Strings(chosen)
	c := Choice{Tests: chosen}
	if len(chosen) == 0 {
		c.Note = "no test touches the changed packages"
	}
	return c
}

func diff(tests []string, mapped map[string][]string) (added, removed []string) {
	now := map[string]bool{}
	for _, t := range tests {
		now[t] = true
		if _, ok := mapped[t]; !ok {
			added = append(added, t)
		}
	}
	for t := range mapped {
		if !now[t] {
			removed = append(removed, t)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// TouchedPackages reads `go tool covdata textfmt` output and keeps the
// in-module packages with any counter above zero. covdata's pkglist is no
// use here: it lists every package instrumented, run or not.
func TouchedPackages(profile, module string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(profile, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.HasPrefix(fields[0], module+"/") {
			continue
		}
		if n, err := strconv.Atoi(fields[2]); err != nil || n == 0 {
			continue
		}
		file, _, _ := strings.Cut(fields[0], ":")
		seen[path.Dir(file)] = true
	}
	var pkgs []string
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}
