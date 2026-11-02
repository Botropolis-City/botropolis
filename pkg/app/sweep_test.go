package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
)

// Item 333: a city pointed at another home with --home writes nothing,
// so its quiet sweep is refused before the daemon is asked.
func TestSweeper(t *testing.T) {
	t.Run("when the run was pointed at a home", func(t *testing.T) {
		socket := filepath.Join(t.TempDir(), "never-dialled.sock")
		_, err := sweeper(&config.Config{HomeSet: true, Socket: socket})([]string{"a"}, time.Hour)

		t.Run("it should refuse, saying why", func(t *testing.T) {
			assert.EqualError(t, err, "the quiet sweep packs up only your own sessions, not those of a home --home points at")
		})
	})

	t.Run("when no daemon is listening", func(t *testing.T) {
		socket := filepath.Join(t.TempDir(), "nobody.sock")
		_, err := sweeper(&config.Config{Socket: socket})([]string{"a"}, time.Hour)

		t.Run("it should say the sweep needs the daemon", func(t *testing.T) {
			assert.ErrorContains(t, err, "packing up needs the daemon")
		})
	})
}
