package group

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 197: a group asked for again reads what Order reads, without
// reading again unless its sites changed.
func TestOrders(t *testing.T) {
	t.Run("when groups generated from seeds 1 to 100 are each asked for twice, with every site's done flipped between", func(t *testing.T) {
		var o Orders
		for seed := uint64(1); seed <= 100; seed++ {
			o.Order(fmt.Sprint(seed), generatedSites(seed))
		}
		o.Sweep()
		reads := o.Reads
		for seed := uint64(1); seed <= 100; seed++ {
			sites := generatedSites(seed)
			for i := range sites {
				sites[i].Done = !sites[i].Done
			}
			wantOrder, wantWaits := Order(sites)
			gotOrder, gotWaits := o.Order(fmt.Sprint(seed), sites)
			t.Run(fmt.Sprintf("seed %d, it should read the same order", seed), func(t *testing.T) {
				assert.Equal(t, wantOrder, gotOrder)
			})
			t.Run(fmt.Sprintf("seed %d, it should read the same waits", seed), func(t *testing.T) {
				assert.Equal(t, wantWaits, gotWaits)
			})
		}
		t.Run("it should not read any of them again", func(t *testing.T) {
			assert.Equal(t, reads, o.Reads)
		})
	})

	t.Run("when a group's words change between askings", func(t *testing.T) {
		var o Orders
		sites := generatedSites(7)
		o.Order("g", sites)
		o.Sweep()
		sites[0].Words = append(sites[0].Words, "waiting on "+sites[1].Name)
		o.Order("g", sites)

		t.Run("it should read it again", func(t *testing.T) {
			assert.Equal(t, 2, o.Reads)
		})
	})

	t.Run("when a group was not asked for before the last sweep", func(t *testing.T) {
		var o Orders
		o.Order("g", generatedSites(7))
		o.Sweep()
		o.Sweep()
		o.Order("g", generatedSites(7))

		t.Run("it should read it again", func(t *testing.T) {
			assert.Equal(t, 2, o.Reads)
		})
	})
}
