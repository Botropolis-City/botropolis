package group_test

import (
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
	before := history(map[string][]string{
		"a": {"add rate-limit headers to the limiter"},
		"b": {"read the headers in the middleware"},
	})
	ids := []string{"a", "b", "filler-00", "filler-01"}
	cases := []struct {
		scenario string
		after    map[string][]string
		muted    []string
	}{
		{scenario: "when a session's prompts grow", after: history(map[string][]string{
			"a": {"add rate-limit headers to the limiter"},
			"b": {"read the headers in the middleware", "and the rate-limit too"},
		})},
		{scenario: "when a session's earlier prompts change", after: history(map[string][]string{
			"a": {"add quota headers to the limiter"},
			"b": {"read the headers in the middleware"},
		})},
		{scenario: "when a word is muted", after: before, muted: []string{"headers"}},
		{scenario: "when a session is gone", after: history(map[string][]string{
			"a": {"add rate-limit headers to the limiter"},
		})},
	}
	for _, c := range cases {
		t.Run(c.scenario, func(t *testing.T) {
			got := group.NewWording(before, nil).Next(c.after, c.muted)

			t.Run("it should weigh words as a wording built from scratch does", func(t *testing.T) {
				assert.Equal(t, weighed(group.NewWording(c.after, c.muted), ids), weighed(got, ids))
			})
		})
	}
}
