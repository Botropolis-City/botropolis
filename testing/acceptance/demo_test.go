package acceptance_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/demo"
	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/testing/helpers"
)

// isDigit is a number key that once chose one of nine views; since
// item 88 only the first six choose a lens (item 177).
var isDigit = regexp.MustCompile(`^Digit[1-9]$`)

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

			targets := map[string]bool{"plant": true, "home": true}
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
				for _, key := range keys {
					if !isDigit.MatchString(key) {
						continue
					}
					lens, selects := render.LensOfKey(key)
					t.Run("it should select a lens with "+shot.Name+"'s "+key, func(t *testing.T) {
						assert.True(t, selects)
					})
					if !selects {
						continue
					}
					if len(shot.Script) == 0 {
						t.Run("it should title the still "+shot.Name+" for the lens "+key+" selects, "+lens.Name(), func(t *testing.T) {
							assert.Contains(t, strings.ToLower(shot.Title), strings.ToLower(lens.Name()))
						})
					} else {
						t.Run("it should name in "+shot.Name+"'s description the lens "+key+" selects, "+lens.Name(), func(t *testing.T) {
							assert.Contains(t, strings.ToLower(shot.Description), strings.ToLower(lens.Name()))
						})
					}
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
// this rather than replaying a pass taken against a different tree. The
// token list is opened even when it is absent, so its arriving re-runs it.
func TestDemoCorpusIsClean(t *testing.T) {
	root := filepath.Join("..", "..")
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "demo"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		_, err = os.ReadFile(p)
		return err
	}))
	_, _ = os.ReadFile(filepath.Join(root, ".publishable-tokens"))
	cmd := exec.Command(helpers.Tool(t, "check-demo-corpus"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()

	t.Run("when the demo directory is checked", func(t *testing.T) {
		t.Run("it should find nothing from a real machine and nothing unreviewed", func(t *testing.T) {
			assert.NoError(t, err, string(out))
		})
	})
}

// checkTeamCorpus runs the corpus check on a demo directory of its own,
// holding a teammate whose team is session-7e4d0b2a and, when withLead,
// that team's lead. Every file is on REVIEWED, so only the team rule can
// fail.
func checkTeamCorpus(t *testing.T, withLead bool) (string, error) {
	t.Helper()
	files := map[string]string{
		"tidepool/9c1f5e3d-0000-4000-8000-000000000002.jsonl": `{"type":"user","cwd":"/home/demo/src/tidepool","teamName":"session-7e4d0b2a","agentName":"reader"}`,
	}
	if withLead {
		files["tidepool/7e4d0b2a-0000-4000-8000-000000000001.jsonl"] = `{"type":"user","cwd":"/home/demo/src/tidepool","teamName":"session-7e4d0b2a"}`
	}
	return checkCorpus(t, files)
}

// checkCorpus runs the corpus check on a demo directory holding only
// these files, every one on REVIEWED.
func checkCorpus(t *testing.T, files map[string]string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	var reviewed strings.Builder
	for rel, content := range files {
		path := filepath.Join(dir, "corpus", rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0o600))
		sum := sha256.Sum256([]byte(content + "\n"))
		reviewed.WriteString(hex.EncodeToString(sum[:]) + "  " + rel + "\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corpus", "REVIEWED"), []byte(reviewed.String()), 0o600))
	cmd := exec.Command(helpers.Tool(t, "check-demo-corpus"), "--root", dir)
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestDemoCorpusChecksTeams(t *testing.T) {
	t.Run("when a teammate's transcript names a team whose lead is in the corpus", func(t *testing.T) {
		out, err := checkTeamCorpus(t, true)

		t.Run("it should pass", func(t *testing.T) {
			assert.NoError(t, err, out)
		})
	})

	t.Run("when a teammate's transcript names a team whose lead is not in the corpus", func(t *testing.T) {
		out, err := checkTeamCorpus(t, false)

		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, err)
		})
		t.Run("it should name the transcript and the missing lead", func(t *testing.T) {
			assert.Regexp(t, `9c1f5e3d-0000-4000-8000-000000000002\.jsonl: .*session-7e4d0b2a.*no lead`, out)
		})
		t.Run("it should say staging cannot write the team's config without it", func(t *testing.T) {
			assert.Contains(t, out, "staging cannot write the team's config without its lead")
		})
	})
}

// The scrub keeps only the lines the app reads, so a corpus line of any
// other type was recorded before it, or past it (item 105).
func TestDemoCorpusRefusesLinesTheAppDoesNotRead(t *testing.T) {
	const transcript = "tidepool/7e4d0b2a-0000-4000-8000-000000000001.jsonl"
	for _, unread := range []string{
		`{"type":"api-request-shape","shape":{}}`,
		`{"type":"attachment","attachment":{"type":"skill_listing"}}`,
	} {
		t.Run("when a corpus transcript holds "+unread, func(t *testing.T) {
			out, err := checkCorpus(t, map[string]string{transcript: `{"type":"user","cwd":"/home/demo/src/tidepool"}` + "\n" + unread})

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err)
			})
			t.Run("it should name the transcript and the line it should not hold", func(t *testing.T) {
				assert.Regexp(t, regexp.QuoteMeta(transcript)+`: line 2 is .*the app does not read`, out)
			})
		})
	}
}
