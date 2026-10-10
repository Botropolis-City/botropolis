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
	opened := func(t *testing.T, sessions ...state.Session) (*city.Scene, city.Card) {
		t.Helper()
		s := selected(t, sessions...)
		s.Act(city.ActionSummary)
		card, _, ok := s.SelectedCard()
		require.True(t, ok)
		return s, card
	}
	recapped := session("a", cinders, state.NeedsYou)
	recapped.Recap = claude.Recap{Text: "We shipped the button. Next: the changelog.", At: now.Add(-2 * time.Hour), PromptsSince: 3}
	bare := session("a", cinders, state.NeedsYou)
	bare.LastPrompt = "add a summary button"

	t.Run("when a building is selected and its summary is closed", func(t *testing.T) {
		s := selected(t, recapped)
		card, _, _ := s.SelectedCard()

		t.Run("it should offer the summary after the session's own actions", func(t *testing.T) {
			assert.Equal(t, []city.ActionKind{city.ActionAttach, city.ActionStop, city.ActionSummary}, s.Actions()[:3])
		})

		t.Run("it should carry no paragraph", func(t *testing.T) {
			assert.Empty(t, card.Para)
		})
	})

	t.Run("when the summary is opened on a session with a recap", func(t *testing.T) {
		s, card := opened(t, recapped)

		t.Run("it should show the recap as the paragraph", func(t *testing.T) {
			assert.Equal(t, "We shipped the button. Next: the changelog.", card.Para)
		})

		t.Run("it should say how old the recap is and how far the session has moved", func(t *testing.T) {
			assert.Contains(t, card.Lines, "recap    2h00m old, 3 turns since")
		})

		t.Run("it should offer to hide it", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionSummaryHide)
		})

		t.Run("it should not offer to generate one", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionGenerate)
		})

		t.Run("and it is hidden again", func(t *testing.T) {
			s.Act(city.ActionSummaryHide)
			card, _, _ := s.SelectedCard()

			t.Run("it should drop the paragraph", func(t *testing.T) {
				assert.Empty(t, card.Para)
			})
		})
	})

	t.Run("when the recap was the session's last word", func(t *testing.T) {
		quiet := recapped
		quiet.Recap.PromptsSince = 0
		_, card := opened(t, quiet)

		t.Run("it should say nothing has happened since", func(t *testing.T) {
			assert.Contains(t, card.Lines, "recap    2h00m old, nothing since")
		})
	})

	t.Run("when the summary is opened on a session with no recap", func(t *testing.T) {
		s, card := opened(t, bare)

		t.Run("it should say there is none yet", func(t *testing.T) {
			assert.Contains(t, card.Lines, "recap    none yet; the last prompt was")
		})

		t.Run("it should fall back to the last prompt", func(t *testing.T) {
			assert.Equal(t, "add a summary button", card.Para)
		})

		t.Run("it should offer to generate one", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})

		t.Run("it should ask the actor to generate one for this session", func(t *testing.T) {
			action, _ := s.Act(city.ActionGenerate)
			assert.Equal(t, city.Action{Kind: city.ActionGenerate, SessionID: "a"}, action)
		})
	})

	t.Run("when a summary is being generated", func(t *testing.T) {
		s := selected(t, bare)
		s.Act(city.ActionSummary)
		s.SetGenerated("a", city.Generated{Pending: true})
		card, _, _ := s.SelectedCard()

		t.Run("it should say so", func(t *testing.T) {
			assert.Contains(t, card.Lines, "summary  generating with "+claude.SummaryModel+"…")
		})

		t.Run("it should not offer a second run", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when a summary has been generated", func(t *testing.T) {
		s := selected(t, bare)
		s.Act(city.ActionSummary)
		s.SetGenerated("a", city.Generated{Text: "A generated summary.", At: now.Add(-5 * time.Minute)})
		card, _, _ := s.SelectedCard()

		t.Run("it should show it as the paragraph", func(t *testing.T) {
			assert.Equal(t, "A generated summary.", card.Para)
		})

		t.Run("it should label it as generated, with its age", func(t *testing.T) {
			assert.Contains(t, card.Lines, "summary  generated 5m ago with "+claude.SummaryModel)
		})

		t.Run("it should offer to generate a fresh one", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when generating failed", func(t *testing.T) {
		s := selected(t, bare)
		s.Act(city.ActionSummary)
		s.SetGenerated("a", city.Generated{Err: "claude: not logged in"})
		card, _, _ := s.SelectedCard()

		t.Run("it should say why", func(t *testing.T) {
			assert.Contains(t, card.Lines, "summary  failed: claude: not logged in")
		})

		t.Run("it should offer to try again", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionGenerate)
		})
	})

	t.Run("when the summary is open and another building is selected", func(t *testing.T) {
		other := session("b", botropolis, state.Working)
		s, _ := opened(t, recapped, other)
		s.Click(centreOf(t, s, "b"))

		t.Run("it should start closed on the new selection", func(t *testing.T) {
			assert.Contains(t, s.Actions(), city.ActionSummary)
		})
	})

	t.Run("when the selected session is not Claude Code's", func(t *testing.T) {
		codex := session("a", cinders, state.NeedsYou)
		codex.Harness = "codex"
		s := selected(t, codex)

		t.Run("it should offer no summary", func(t *testing.T) {
			assert.NotContains(t, s.Actions(), city.ActionSummary)
		})
	})

	t.Run("when the summary key is pressed", func(t *testing.T) {
		t.Run("and nothing is selected, it should say to select something", func(t *testing.T) {
			_, note := scene(t, recapped).ToggleSummary()
			assert.Equal(t, "select a building first", note)
		})

		t.Run("and a building is selected, it should open its summary", func(t *testing.T) {
			s := selected(t, recapped)
			s.ToggleSummary()
			assert.Contains(t, s.Actions(), city.ActionSummaryHide)
		})

		t.Run("and it is pressed again, it should close it", func(t *testing.T) {
			s := selected(t, recapped)
			s.ToggleSummary()
			s.ToggleSummary()
			assert.Contains(t, s.Actions(), city.ActionSummary)
		})

		t.Run("and the session is not Claude Code's, it should say there is none", func(t *testing.T) {
			codex := session("a", cinders, state.NeedsYou)
			codex.Harness = "codex"
			_, note := selected(t, codex).ToggleSummary()
			assert.Equal(t, "no summary for codex sessions", note)
		})
	})
}
