package helpers

import (
	"time"

	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/demo"
)

// HistoryPrompt is one line of a synthetic prompt history; the
// generators live in pkg/demo so botropolis-demo synth-home can make the
// same homes outside go test (item 248).
type HistoryPrompt = demo.HistoryPrompt

func SyntheticHistory(seed int64, sessions, prompts int, start time.Time) []HistoryPrompt {
	return demo.SyntheticHistory(seed, sessions, prompts, start)
}

func ManySessions(seed int64, n int, end time.Time) []HistoryPrompt {
	return demo.ManySessions(seed, n, end)
}

// WriteHistory writes a history into the home: a transcript for each
// session, timed in layout, and every prompt in history.jsonl.
func (h *Home) WriteHistory(history []HistoryPrompt, layout string) *Home {
	h.t.Helper()
	_, err := demo.WriteHome(h.Path, history, layout)
	require.NoError(h.t, err)
	return h
}
