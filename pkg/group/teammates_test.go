package group_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/group"
)

func TestTeammatePRs(t *testing.T) {
	t0 := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	const url = "https://code.example.com/acme/lib/pull/7"
	own := []group.SitePR{{Repository: "acme/api", Number: 882, URL: "https://code.example.com/acme/api/pull/882"}}
	words := func(site int, text string, at time.Duration) group.Mention {
		return group.Mention{Site: site, Text: text, At: t0.Add(at)}
	}
	ran := func(site int, text string, at time.Duration) group.Mention {
		m := words(site, text, at)
		m.Command = true
		return m
	}

	t.Run("when a session's words name a pull request no session in the group linked", func(t *testing.T) {
		got := group.TeammatePRs(own, []group.Mention{words(1, "waiting on "+url, 0)})
		require.Len(t, got, 1)

		t.Run("it should be a pull request opened outside these sessions", func(t *testing.T) {
			assert.Equal(t, group.SitePR{Repository: "acme/lib", Number: 7, URL: url}, got[0].PR)
		})

		t.Run("it should carry the site that mentioned it", func(t *testing.T) {
			assert.Equal(t, 1, got[0].Site)
		})
	})

	t.Run("when the pull request named is one a group session linked", func(t *testing.T) {
		t.Run("it should not be one opened outside", func(t *testing.T) {
			assert.Empty(t, group.TeammatePRs(own, []group.Mention{words(0, "see "+own[0].URL, 0)}))
		})
	})

	for _, c := range []struct {
		when             string
		line             group.Mention
		merged, isClosed bool
	}{
		{"only words say the pull request is merged", words(1, "once "+url+" is merged, bump it", 0), false, false},
		{"a command merged it", ran(1, "gh pr merge "+url+" --squash", 0), true, false},
		{"a command closed it", ran(1, "gh pr close "+url, 0), false, true},
	} {
		t.Run("when "+c.when, func(t *testing.T) {
			got := group.TeammatePRs(own, []group.Mention{words(1, "see "+url, -time.Hour), c.line})
			require.Len(t, got, 1)

			t.Run("its state should be read only from what was run", func(t *testing.T) {
				assert.Equal(t, [2]bool{c.merged, c.isClosed}, [2]bool{got[0].PR.Merged, got[0].Closed})
			})
		})
	}

	t.Run("when two sessions name the same pull request", func(t *testing.T) {
		got := group.TeammatePRs(own, []group.Mention{words(0, "see "+url, -2*time.Hour), words(1, "see "+url, -time.Hour)})

		t.Run("it should be one pull request", func(t *testing.T) {
			assert.Len(t, got, 1)
		})

		t.Run("it should be seen last at the newer mention", func(t *testing.T) {
			assert.Equal(t, [2]any{1, t0.Add(-time.Hour)}, [2]any{got[0].Site, got[0].Seen})
		})
	})
}
