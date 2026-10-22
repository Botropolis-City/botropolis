package group_test

import (
	"sort"
	"testing"

	"botropolis.city/botropolis/pkg/group"
	"github.com/stretchr/testify/assert"
)

func TestWordingNext(t *testing.T) {
	weighed := func(w *group.Wording, ids []string) map[string]any {
		out := map[string]any{}
		for _, a := range ids {
			for _, b := range ids {
				words, score := w.Shared(a, b)
				out[a+"|"+b] = []any{words, score, w.Related(a, b)}
			}
			out[a] = w.Top([]string{a}, 5)
		}
		return out
	}
	movedBetween := func(before, after *group.Wording) []string {
		moved := map[string]bool{}
		for tok, idf := range after.Rarities() {
			if was, ok := before.Rarities()[tok]; !ok || was != idf {
				moved[tok] = true
			}
		}
		for tok := range before.Rarities() {
			if _, ok := after.Rarities()[tok]; !ok {
				moved[tok] = true
			}
		}
		out := []string{}
		for tok := range moved {
			out = append(out, tok)
		}
		sort.Strings(out)
		return out
	}
	before := history(map[string][]string{
		"a": {"add rate-limit headers to the limiter"},
		"b": {"read the headers in the middleware"},
		"c": {"zeta"}, "d": {"zeta"}, "e": {"zeta"}, "f": {"zeta"}, "g": {"eta"},
	})
	ids := []string{"a", "b", "c", "g", "filler-00", "filler-01"}
	with := func(changes map[string][]string, gone ...string) map[string][]string {
		out := map[string][]string{}
		for id, ps := range before {
			out[id] = ps
		}
		for id, ps := range changes {
			out[id] = ps
		}
		for _, id := range gone {
			delete(out, id)
		}
		return out
	}
	cases := []struct {
		scenario string
		after    map[string][]string
		muted    []string
		allMoved bool
	}{
		{scenario: "when a session's prompts grow",
			after: with(map[string][]string{"b": {"read the headers in the middleware", "and the rate-limit too"}})},
		{scenario: "when a session's earlier prompts change",
			after: with(map[string][]string{"a": {"add quota headers to the limiter"}})},
		{scenario: "when a word becomes too common to weigh",
			after: with(map[string][]string{"g": {"eta zeta"}})},
		{scenario: "when a word becomes rare enough to weigh",
			after: with(map[string][]string{"f": {"theta"}})},
		{scenario: "when a word is no longer used",
			after: with(map[string][]string{"g": {"iota"}})},
		{scenario: "when nothing changes", after: before},
		{scenario: "when a word is muted", after: before, muted: []string{"headers"}, allMoved: true},
		{scenario: "when a session is gone", after: with(nil, "b"), allMoved: true},
		{scenario: "when a session joins", after: with(map[string][]string{"h": {"kappa"}}), allMoved: true},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			got := group.NewWording(before, nil).Next(c.after, c.muted)
			fresh := group.NewWording(c.after, c.muted)
			all, moved := got.Moved()

			t.Run("it should weigh words as a wording built from scratch does", func(t *testing.T) {
				assert.Equal(t, weighed(fresh, ids), weighed(got, ids))
			})

			t.Run("it should hold every word's rarity as a wording built from scratch does", func(t *testing.T) {
				assert.Equal(t, fresh.Rarities(), got.Rarities())
			})

			t.Run("it should count every word's sessions as a wording built from scratch does", func(t *testing.T) {
				assert.Equal(t, fresh.Counts(), got.Counts())
			})

			if c.allMoved {
				t.Run("it should report every word moved", func(t *testing.T) {
					assert.True(t, all)
				})
			} else {
				t.Run("it should report exactly the words whose rarity moved", func(t *testing.T) {
					assert.Equal(t, movedBetween(group.NewWording(before, nil), fresh), moved)
				})
			}
		})
	}
}
