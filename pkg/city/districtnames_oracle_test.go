package city_test

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
)

// The district names as they were found before item 201, kept as the
// oracle the faster finding must agree with: each root against every
// other, on every snapshot.
func oracleDistrictNames(roots []string) map[string]string {
	split := map[string][]string{}
	for _, root := range roots {
		if root != "" {
			split[root] = strings.Split(strings.Trim(filepath.ToSlash(root), "/"), "/")
		}
	}
	endsElsewhere := func(root string, tail []string) bool {
		for other, parts := range split {
			if other != root && len(parts) >= len(tail) && slices.Equal(parts[len(parts)-len(tail):], tail) {
				return true
			}
		}
		return false
	}
	names := make(map[string]string, len(roots))
	for _, root := range roots {
		if root == "" {
			names[root] = city.FolderUnknown
			continue
		}
		names[root] = root
		parts := split[root]
		for k := 1; k <= len(parts); k++ {
			tail := parts[len(parts)-k:]
			if !endsElsewhere(root, tail) {
				names[root] = strings.Join(tail, "/")
				break
			}
		}
	}
	return names
}

// Item 201: a snapshot that changes one session's activity laid the
// city out in 7 ms, a fifth of it naming the districts by comparing
// every root with every other.
func TestDistrictNamesAgreeWithTheOracle(t *testing.T) {
	words := []string{"src", "work", "api", "web", "cli", "app", "home", "user", "aria", "github", "botropolis", "a", "b"}
	r := rand.New(rand.NewPCG(201, 1))
	for n := range 400 {
		var roots []string
		for range 1 + r.IntN(40) {
			var parts []string
			for range r.IntN(6) {
				parts = append(parts, words[r.IntN(len(words))])
			}
			root := "/" + strings.Join(parts, "/")
			if r.IntN(10) == 0 {
				root = ""
			}
			if r.IntN(10) == 0 {
				root += "/"
			}
			roots = append(roots, root)
		}
		if r.IntN(5) == 0 && len(roots) > 1 {
			roots = append(roots, roots[0])
		}
		t.Run(fmt.Sprintf("when %d roots are named, set %d", len(roots), n), func(t *testing.T) {
			t.Run("it should name them as the oracle does", func(t *testing.T) {
				assert.Equal(t, oracleDistrictNames(roots), city.DistrictNames(roots))
			})
		})
	}
}

// What naming the districts of item 197's city costs, as it was and as
// it is. It reports and never gates.
func BenchmarkDistrictNames(b *testing.B) {
	var roots []string
	for i := range 200 {
		roots = append(roots, fmt.Sprintf("/home/user/src/project-%03d", i%150))
	}
	b.Run("oracle", func(b *testing.B) {
		for b.Loop() {
			oracleDistrictNames(roots)
		}
	})
	b.Run("now", func(b *testing.B) {
		for b.Loop() {
			city.DistrictNames(roots)
		}
	})
}
