package city_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestSelectionSummary(t *testing.T) {
	selected := func(t *testing.T, sessions ...state.Session) *city.Scene {
		t.Helper()
		s := scene(t, sessions...)
		s.Click(centreOf(t, s, sessions[0].ID))
		require.NotNil(t, s.Selected())
		return s
	}
	details := func(t *testing.T, s *city.Scene) []string {
		t.Helper()
		card, _, ok := s.SelectedCard()
		require.True(t, ok)
		return card.Details
	}
	recapped := session("a", cinders, state.Waiting)
	recapped.Recap = claude.Recap{Text: "We shipped the button. Next: the changelog.", At: now.Add(-2 * time.Hour), PromptsSince: 3}
	bare := session("a", cinders, state.Waiting)
	bare.LastPrompt = "add a summary button"

	t.Run("when the selected session has a recap", func(t *testing.T) {
		s := selected(t, recapped)

		t.Run("it should say in its details how old the recap is and how far the session has moved", func(t *testing.T) {
			assert.Contains(t, details(t, s), "recap    2h00m old, 3 turns since")
		})

		t.Run("it should not offer to generate one", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when the recap was the session's last word", func(t *testing.T) {
		quiet := recapped
		quiet.Recap.PromptsSince = 0

		t.Run("it should say nothing has happened since", func(t *testing.T) {
			assert.Contains(t, details(t, selected(t, quiet)), "recap    2h00m old, nothing since")
		})
	})

	t.Run("when the selected session has no recap", func(t *testing.T) {
		s := selected(t, bare)

		t.Run("it should say there is none yet", func(t *testing.T) {
			assert.Contains(t, details(t, s), "recap    none yet")
		})

		t.Run("it should ask the actor to generate one for this session", func(t *testing.T) {
			action, _ := s.Act(city.ActionGenerate)
			assert.Equal(t, city.Action{Kind: city.ActionGenerate, SessionID: "a"}, action)
		})
	})

	t.Run("when a summary is being generated", func(t *testing.T) {
		s := selected(t, bare)
		s.SetGenerated("a", city.Generated{Pending: true})

		t.Run("it should say so", func(t *testing.T) {
			assert.Contains(t, details(t, s), "summary  generating with "+claude.SummaryModel+"…")
		})

		t.Run("it should not offer a second run", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when a summary has been generated", func(t *testing.T) {
		s := selected(t, bare)
		s.SetGenerated("a", city.Generated{Text: "A generated summary.", At: now.Add(-5 * time.Minute)})

		t.Run("it should label it as generated, with its age", func(t *testing.T) {
			assert.Contains(t, details(t, s), "summary  generated 5m ago with "+claude.SummaryModel)
		})

		t.Run("it should offer to generate a fresh one", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when generating failed", func(t *testing.T) {
		s := selected(t, bare)
		s.SetGenerated("a", city.Generated{Err: "claude: not logged in"})

		t.Run("it should say why", func(t *testing.T) {
			assert.Contains(t, details(t, s), "summary  failed: claude: not logged in")
		})

		t.Run("it should offer to try again", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when the selected session is not Claude Code's", func(t *testing.T) {
		codex := bare
		codex.Harness = "codex"
		s := selected(t, codex)

		t.Run("it should offer no summary", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionGenerate)
		})

		t.Run("it should say nothing about a recap", func(t *testing.T) {
			assert.NotContains(t, details(t, s), "recap    none yet")
		})
	})

	t.Run("when the details key is pressed", func(t *testing.T) {
		t.Run("and nothing is selected, it should say to select something", func(t *testing.T) {
			_, note := scene(t, recapped).ToggleDetails()
			assert.Equal(t, "select a building first", note)
		})

		t.Run("and a building is selected, it should fold its details out", func(t *testing.T) {
			s := selected(t, recapped)
			s.ToggleDetails()
			assert.Contains(t, s.Actions(), city.ActionDetailsHide)
		})

		t.Run("and it is pressed again, it should fold them away", func(t *testing.T) {
			s := selected(t, recapped)
			s.ToggleDetails()
			s.ToggleDetails()
			assert.Contains(t, s.Actions(), city.ActionDetails)
		})
	})
}
