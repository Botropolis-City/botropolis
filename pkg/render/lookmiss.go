package render

import (
	"reflect"
	"strings"
)

// lookChanged names the fields of a look's key that differ from the key
// its run had last, in the key's order: "new" when it had none, and
// "unchanged" when the key held but its run was not drawn the frame
// before (item 202).
func lookChanged(was lookKey, had bool, now lookKey) string {
	if !had {
		return "new"
	}
	a, b := reflect.ValueOf(was), reflect.ValueOf(now)
	var names []string
	for i := range a.NumField() {
		if !a.Field(i).Equal(b.Field(i)) {
			names = append(names, a.Type().Field(i).Name)
		}
	}
	if len(names) == 0 {
		return "unchanged"
	}
	return strings.Join(names, ",")
}
