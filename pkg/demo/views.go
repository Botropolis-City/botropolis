package demo

import (
	"fmt"
	"slices"
	"strconv"

	"botropolis.city/botropolis/pkg/city"
)

// viewKeys is what a person presses to reach a view from a fresh city:
// its lens's number, once to open the lens on its first view and once
// more for each view after it (item 76).
func viewKeys(name string) ([]string, error) {
	v, ok := city.ViewByName(name)
	if !ok || v.Name() != name {
		return nil, fmt.Errorf("view %q: the city has no such view", name)
	}
	lens := city.LensOf(v)
	key := "Digit" + strconv.Itoa(slices.Index(city.Lenses, lens)+1)
	return slices.Repeat([]string{key}, slices.Index(lens.Views(), v)+1), nil
}
