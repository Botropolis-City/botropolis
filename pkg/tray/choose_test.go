package tray

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 293: StatusNotifierItem where a watcher with a host answers, the
// XEmbed tray where only a manager owns the selection, and neither says
// so, naming both.
func TestChoose(t *testing.T) {
	for _, c := range []struct {
		name          string
		found         found
		want          kind
		wantErrNaming []string
	}{
		{"when a watcher with a host answers, and an XEmbed manager is also present, it should choose StatusNotifierItem",
			found{watcher: true, host: true, manager: true}, kindSNI, nil},
		{"when a watcher with a host answers, and no XEmbed manager is present, it should choose StatusNotifierItem",
			found{watcher: true, host: true}, kindSNI, nil},
		{"when a watcher answers with no host registered, it should fall to XEmbed",
			found{watcher: true, manager: true}, kindXEmbed, nil},
		{"when no watcher answers, and an XEmbed manager owns the selection, it should choose XEmbed",
			found{manager: true}, kindXEmbed, nil},
		{"when no watcher answers, and nothing owns the selection, it should say there is no tray, naming both",
			found{}, kindNone, []string{"StatusNotifierItem", "XEmbed"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := choose(c.found)
			if c.wantErrNaming == nil {
				assert.Equal(t, c.want, got)
				return
			}
			for _, word := range c.wantErrNaming {
				assert.ErrorContains(t, err, word)
			}
		})
	}
}
