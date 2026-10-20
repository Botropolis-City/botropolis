package render

import (
	"fmt"
	"reflect"
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
		{"Journal", "the journal: where you were, and what is next", "shift+w", ""},
		{"Stream", "the stream: what is happening, newest first", "l", ""},
		{"New", "New session, in any directory (also on the board)", "shift+n", ""},
		{"Explain", "caption everything on the map", "e", "hold "},
		{"Plan", "the City Plan: every district on one screen; click a badge to fly in", "m", ""},
		{"Labels", "labels off and on", "z", ""},
	}

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

	t.Run("when every configurable key is rebound", func(t *testing.T) {
		var chords config.Chords
		value := reflect.ValueOf(&chords).Elem()
		for i := 0; i < value.NumField(); i++ {
			value.Field(i).Set(reflect.ValueOf(config.Chord{Alt: true, Key: fmt.Sprintf("f%d", i+1)}))
		}
		keys := (&Game{chords: &chords}).helpKeys()

		for i := 0; i < value.NumField(); i++ {
			name := value.Type().Field(i).Name
			chord := value.Field(i).Interface().(config.Chord).String()

			t.Run("it should name "+name+"'s chord, "+chord+", in the reference", func(t *testing.T) {
				var named bool
				for _, k := range keys {
					named = named || k.Key == chord || strings.HasSuffix(k.Key, " "+chord)
				}
				assert.True(t, named, "%v", keys)
			})
		}

		for _, r := range rows {
			chord := value.FieldByName(r.field).Interface().(config.Chord).String()

			t.Run("it should name "+r.prefix+chord+" for "+r.field, func(t *testing.T) {
				assert.Contains(t, keys, ui.Key{Key: r.prefix + chord, Action: r.action})
			})
		}
	})
}
