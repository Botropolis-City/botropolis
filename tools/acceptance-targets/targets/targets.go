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

// MaxBehind is how many commits a map may be behind HEAD before the full
// suite runs and makes a new one (item 226, slice 3): at least one landing in
// every MaxBehind runs everything, which catches what a selection missed.
const MaxBehind = 30

// AcceptanceDir is the acceptance package, whose own files select tests
// through its call graph rather than through the map.
const AcceptanceDir = "testing/acceptance"

// Situation is what a landing knows when it chooses: what main's CI last
// concluded, the map and how far behind HEAD it is (-1 when it is not an
// ancestor), the package's tests now, and the files changed since the map.
type Situation struct {
	CI      string
	Map     *Map
	Behind  int
	Tests   []string
	Changed []string
}

// Resolver says what a changed file reaches: the package under pkg/ or cmd/
// it belongs to, or, for a file in the acceptance package, the tests that
// reach what changed in it, or why it cannot say.
type Resolver struct {
	PackageOf  func(file string) (string, bool)
	Acceptance func(file string) (tests []string, why string)
}

// Choose picks the acceptance tests a landing runs. It runs them all, saying
// why, whenever it cannot be sure a selection is enough: no map, main's CI
// not green, a map that is not an ancestor or is too far behind, a test added
// or removed, or a change it cannot trace to the tests it reaches.
func Choose(s Situation, r Resolver) Choice {
	m := s.Map
	switch {
	case m == nil:
		return Choice{Fallback: "there is no map yet"}
	case s.CI != "green":
		return Choice{Fallback: fmt.Sprintf("main's CI is %s, so nothing is skipped", s.CI)}
	case s.Behind < 0:
		return Choice{Fallback: "the map at " + m.Rev + " is not an ancestor of HEAD"}
	case s.Behind > MaxBehind:
		return Choice{Fallback: fmt.Sprintf("the map at %s is %d commits behind HEAD, more than %d", m.Rev, s.Behind, MaxBehind)}
	}
	if added, removed := diff(s.Tests, m.Tests); len(added)+len(removed) > 0 {
		return Choice{Fallback: fmt.Sprintf("the tests changed since the map: added %v, removed %v", added, removed)}
	}
	if len(s.Changed) == 0 {
		return Choice{Note: "nothing changed since the map was made at " + m.Rev}
	}
	pkgs := map[string]bool{}
	direct := map[string]bool{}
	for _, f := range s.Changed {
		if path.Dir(filepath.ToSlash(f)) == AcceptanceDir {
			tests, why := r.Acceptance(f)
			if why != "" {
				return Choice{Fallback: why}
			}
			for _, t := range tests {
				direct[t] = true
			}
			continue
		}
		pkg, ok := r.PackageOf(f)
		if !ok {
			return Choice{Fallback: f + " is outside pkg/ and cmd/, so the map cannot say which tests it reaches"}
		}
		pkgs[pkg] = true
	}
	var chosen []string
	for _, test := range s.Tests {
		touched := m.Tests[test]
		if len(touched) == 0 || direct[test] {
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
		c.Note = "no test reaches what changed"
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
