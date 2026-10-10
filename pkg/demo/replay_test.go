package demo

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/state"
)

func TestReplay(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	corpus := t.TempDir()
	turn := func(at string, n, read int) string {
		return `{"type":"assistant","sessionId":"` + recordedID + `","cwd":"` + cwd + `","timestamp":"` + at + `",` +
			`"message":{"id":"m` + strconv.Itoa(n) + `","model":"claude-sonnet-4-5","role":"assistant","content":[{"type":"text","text":"step"}],` +
			`"usage":{"input_tokens":10,"output_tokens":50,"cache_read_input_tokens":` + strconv.Itoa(read) + `,"cache_creation_input_tokens":90}}}`
	}
	writeFile(t, filepath.Join(corpus, "tidepool", recordedID+".jsonl"),
		line("user", "2026-09-01T10:00:00.000Z")+"\n"+
			turn("2026-09-01T10:02:00.000Z", 1, 10000)+"\n"+
			turn("2026-09-01T10:05:00.000Z", 2, 20000)+"\n"+
			turn("2026-09-01T10:10:00.000Z", 3, 30000)+"\n"+
			`{"type":"custom-title","customTitle":"Tide table","sessionId":"`+recordedID+`"}`+"\n")
	scenario := Scenario{Name: "s", Sessions: []Placement{{
		Ref: "tidepool/6f1d", State: state.Working,
		Replay: &Replay{Over: Duration(20 * time.Second), Context: "80%"},
	}}}
	home := t.TempDir()
	d, err := NewDirector(Corpus{Dir: corpus}, scenario, home, now, func() (int, error) { return 7001, nil })
	require.NoError(t, err)
	transcript := filepath.Join(home, ".claude", "projects", claude.ProjectFolder(cwd), recordedID+".jsonl")
	read := func(t *testing.T) claude.Transcript {
		t.Helper()
		tr, err := claude.ReadTranscript(transcript)
		require.NoError(t, err)
		return tr
	}
	until := func(t *testing.T, offset time.Duration) {
		t.Helper()
		for _, e := range d.Events() {
			if e.At <= offset && !d.applied[e.key()] {
				require.NoError(t, d.Apply(e, now.Add(e.At)))
			}
		}
	}

	t.Run("when a replayed session starts", func(t *testing.T) {
		tr := read(t)

		t.Run("it should hold only the opening prompt", func(t *testing.T) {
			assert.EqualValues(t, 0, tr.ContextTokens)
		})

		t.Run("it should carry its title from the start", func(t *testing.T) {
			assert.Equal(t, "Tide table", tr.Title)
		})
	})

	t.Run("when the replay is halfway through", func(t *testing.T) {
		until(t, 10*time.Second)
		half := read(t).ContextTokens

		t.Run("it should have grown, but not all the way", func(t *testing.T) {
			assert.True(t, half > 0 && half < 160000, "context %d", half)
		})

		t.Run("and it finishes", func(t *testing.T) {
			until(t, 20*time.Second)
			tr := read(t)

			t.Run("it should hold the whole recording", func(t *testing.T) {
				assert.Len(t, readLines(t, transcript), 5)
			})

			t.Run("it should end at the context asked for", func(t *testing.T) {
				assert.EqualValues(t, 160000, tr.ContextTokens)
			})

			t.Run("it should end at the moment it was revealed", func(t *testing.T) {
				assert.Equal(t, now.Add(20*time.Second), tr.LastAt)
			})
		})
	})
}
