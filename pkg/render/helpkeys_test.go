package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/ui"
)

// Item 172: the reference names every key as it is configured, and u.
func TestHelpKeys(t *testing.T) {
	rows := []struct {
		field, action, fallback, prefix string
	}{
		{"Triage", "triage: walk what waits on you, longest-waiting first", "w", ""},
		{"Journal", "the journal: where you were, and what is next; r restores an entry's desk", "j", ""},
		{"Stream", "the stream: what is happening, newest first", "l", ""},
		{"New", "New session, filled from what is under the pointer (also on the board)", "shift+n", ""},
		{"Explain", "caption everything on the map", "e", "hold "},
		{"Plan", "the City Plan: every district on one screen; click a badge to fly in", "m", ""},
		{"Labels", "labels off and on", "z", ""},
	}
	bindingOf := map[string]string{"Triage": "triage", "Journal": "journal", "Stream": "stream", "New": "new_session", "Explain": "explain", "Plan": "city_plan", "Labels": "labels"}

	t.Run("when nothing is set", func(t *testing.T) {
		keys := (&Game{}).helpKeys()

		t.Run("it should list u as fetching the usage again", func(t *testing.T) {
			assert.Contains(t, keys, ui.Key{Key: "u", Action: "fetch the plan's usage again; the launch carries it down the river"})
		})

		for _, r := range rows {
			t.Run("it should name "+r.prefix+r.fallback+" for "+r.field, func(t *testing.T) {
				assert.Contains(t, keys, ui.Key{Key: r.prefix + r.fallback, Action: r.action})
			})
		}
	})

	t.Run("when every binding the reference lists is rebound", func(t *testing.T) {
		chords := config.Chords{Bound: map[string]config.Chord{}}
		for i, b := range config.Bindings {
			chords.Bound[b.Name] = config.Chord{Ctrl: true, Alt: true, Key: fmt.Sprintf("f%d", i+1)}
		}
		keys := (&Game{chords: &chords}).helpKeys()

		for _, b := range config.Bindings {
			if b.Scope != config.ScopeMap && b.Scope != config.ScopeEverywhere && b.Scope != config.ScopeFinder {
				continue
			}
			chord := chords.Bound[b.Name].Label()

			t.Run("it should name "+b.Name+"'s chord, "+chord+", in the reference", func(t *testing.T) {
				var named bool
				for _, k := range keys {
					named = named || k.Key == chord || strings.HasSuffix(k.Key, " "+chord) || strings.HasPrefix(k.Key, chord+" ") || strings.Contains(k.Key, " "+chord+" ")
				}
				assert.True(t, named, "%v", keys)
			})
		}

		for _, r := range rows {
			chord := chords.Bound[bindingOf[r.field]].Label()

			t.Run("it should name "+r.prefix+chord+" for "+r.field, func(t *testing.T) {
				assert.Contains(t, keys, ui.Key{Key: r.prefix + chord, Action: r.action})
			})
		}
	})
}
