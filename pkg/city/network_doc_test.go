package city_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
)

// Item 178: design/info-views.md said which networks each view draws, and
// went on saying it after the nine views were folded into six lenses. Its
// table is held against View.Shows so the next fold cannot leave it behind.

type networkRow struct {
	lens, view string
	networks   []string
}

func readNetworkTable(t *testing.T) []networkRow {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "..", "design", "info-views.md"))
	require.NoError(t, err)
	var rows []networkRow
	inTable := false
	for _, line := range strings.Split(string(text), "\n") {
		cells := tableCells(line)
		switch {
		case cells == nil:
			inTable = false
		case len(cells) == 3 && strings.EqualFold(cells[0], "lens") && strings.EqualFold(cells[1], "view") && strings.EqualFold(cells[2], "networks"):
			inTable = true
		case inTable && strings.Trim(cells[0], "-: ") == "":
		case inTable && len(cells) == 3:
			rows = append(rows, networkRow{lens: cells[0], view: strings.Trim(cells[1], "`"), networks: networkNames(cells[2])})
		}
	}
	return rows
}

func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	cells := strings.Split(strings.Trim(line, "|"), "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func networkNames(cell string) []string {
	names := []string{}
	for _, part := range strings.Split(cell, ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		word := strings.Trim(fields[0], "`")
		if word == "—" || word == "none" {
			continue
		}
		names = append(names, word)
	}
	slices.Sort(names)
	return names
}

func TestInfoViewsNetworkTable(t *testing.T) {
	t.Run("when design/info-views.md's network table is read", func(t *testing.T) {
		rows := readNetworkTable(t)
		documented := map[string]networkRow{}
		for _, r := range rows {
			documented[r.lens+"/"+r.view] = r
		}

		for _, lens := range city.Lenses {
			for _, view := range lens.Views() {
				key := lens.Name() + "/" + view.Name()
				t.Run("and the lens is "+lens.Name()+" on its "+view.Name()+" view", func(t *testing.T) {
					row, ok := documented[key]
					t.Run("it should have a row", func(t *testing.T) {
						assert.True(t, ok, "no row for %s", key)
					})
					t.Run("it should list exactly the networks the view shows", func(t *testing.T) {
						want := []string{}
						for _, n := range city.Networks {
							if view.Shows(n) {
								want = append(want, n.String())
							}
						}
						slices.Sort(want)
						assert.Equal(t, want, row.networks)
					})
				})
			}
		}

		t.Run("it should name only lenses and views that exist", func(t *testing.T) {
			known := map[string]bool{}
			for _, lens := range city.Lenses {
				for _, view := range lens.Views() {
					known[lens.Name()+"/"+view.Name()] = true
				}
			}
			var unknown []string
			for _, r := range rows {
				if !known[r.lens+"/"+r.view] {
					unknown = append(unknown, r.lens+"/"+r.view)
				}
			}
			assert.Empty(t, unknown)
		})
	})
}
