package city_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestDistrictSign(t *testing.T) {
	resting := func(prs ...claude.PR) state.Session {
		s := session("a", cinders, state.Resting)
		s.PRs = prs
		return s
	}
	moves := []struct {
		name    string
		session state.Session
		says    string
	}{
		{"yours", session("a", cinders, state.NeedsYou), "your move"},
		{"the agents'", session("a", cinders, state.Working), "the agents' move"},
		{"someone else's", resting(claude.PR{Number: 7, Repository: "o/r", State: claude.PROpen}), "someone else's move"},
		{"nobody's", resting(), "nobody's move"},
	}
	for _, m := range moves {
		t.Run("when the move is "+m.name, func(t *testing.T) {
			d := build(t, city.NewLayout(), m.session).Districts[0]
			sign := d.Sign()

			t.Run("it should say the district's name and that move", func(t *testing.T) {
				assert.Equal(t, []string{"cinders", m.says}, []string{sign.Name, sign.Move})
			})
		})
	}

	t.Run("when a district has a sign", func(t *testing.T) {
		d := build(t, city.NewLayout(), session("a", cinders, state.Working)).Districts[0]
		sign := d.Sign()

		t.Run("it should stand at the district's front corner, inside the district", func(t *testing.T) {
			front := d.Rect.Max
			assert.True(t, d.Rect.Contains(sign.At) && sign.At.X > d.Rect.Center().X && sign.At.Y > d.Rect.Center().Y && sign.At.X < front.X && sign.At.Y < front.Y)
		})
	})
}
