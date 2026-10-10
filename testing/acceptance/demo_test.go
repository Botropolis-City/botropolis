package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/state"
)

// Every scenario must stage from the committed corpus and load as the
// city it says it is. A scenario names sessions by script label, so a
// re-recording that drops a label, or a typo in a scenario, fails here
// rather than in a film run.
func TestDemoScenarios(t *testing.T) {
	root := filepath.Join("..", "..")
	paths, err := filepath.Glob(filepath.Join(root, "demo", "scenarios", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, paths, "no scenarios: every check below passes on nothing")
	probes := state.Probes{Alive: state.ProcessAlive, Attached: state.UnixSocketConnected}

	for _, path := range paths {
		scenario, err := demo.LoadScenario(path)
		require.NoError(t, err)
		want := map[state.State]int{}
		for _, p := range scenario.Sessions {
			if p.Arrive == 0 {
				want[p.State]++
			}
		}

		t.Run("when the "+scenario.Name+" scenario is staged from the corpus", func(t *testing.T) {
			home := t.TempDir()
			now := time.Now()
			corpus := demo.Corpus{Dir: filepath.Join(root, "demo", "corpus")}
			require.NoError(t, demo.Stage(corpus, scenario, home, now, demo.Sleepers(home, time.Minute)))
			t.Cleanup(func() { _ = demo.Unstage(home) })
			snapshot, err := state.Load(home, probes, now)
			require.NoError(t, err)
			got := map[state.State]int{}
			for _, s := range snapshot.Sessions {
				got[s.State]++
			}

			t.Run("it should load with the states the scenario places", func(t *testing.T) {
				assert.Equal(t, want, got)
			})

			targets := map[string]bool{"plant": true}
			for _, s := range snapshot.Sessions {
				targets[s.Title] = true
				targets["district:"+filepath.Base(s.CWD)] = true
			}
			for _, p := range scenario.Sessions {
				if p.Title != "" {
					targets[p.Title] = true
				}
				if p.Ref != "" && p.Arrive > 0 {
					rec, err := corpus.Find(p.Ref)
					require.NoError(t, err)
					tr, err := claude.ReadTranscript(rec.Path)
					require.NoError(t, err)
					targets[tr.Title] = true
					targets["district:"+rec.Project] = true
				}
				if p.Project != "" {
					targets["district:"+p.Project] = true
				}
			}
			for _, shot := range scenario.Shots {
				keys := append([]string(nil), shot.Keys...)
				for _, cue := range shot.Script {
					keys = append(keys, cue.Key, cue.Hold)
				}
				for _, key := range keys {
					if key == "" {
						continue
					}
					t.Run("it should know the key "+shot.Name+" presses, "+key, func(t *testing.T) {
						assert.True(t, render.KnownKey(key))
					})
				}
				for _, cue := range shot.Script {
					for _, target := range []string{cue.PointAt, cue.PanTo} {
						if target == "" {
							continue
						}
						t.Run("it should find what "+shot.Name+" aims at, "+target, func(t *testing.T) {
							assert.True(t, targets[target])
						})
					}
				}
			}
		})
	}
}

// The corpus guard is run by make lint; this is its second caller, so
// it survives losing the Makefile line. Opening every file the guard
// reads puts them in the test cache's key, so a corpus change re-runs
// this rather than replaying a pass taken against a different tree.
func TestDemoCorpusIsClean(t *testing.T) {
	root := filepath.Join("..", "..")
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "demo"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		_, err = os.ReadFile(p)
		return err
	}))
	cmd := exec.Command("tools/check-demo-corpus")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()

	t.Run("when the demo directory is checked", func(t *testing.T) {
		t.Run("it should find nothing from a real machine and nothing unreviewed", func(t *testing.T) {
			assert.NoError(t, err, string(out))
		})
	})
}
