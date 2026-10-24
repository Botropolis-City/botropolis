package demo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShotKey(t *testing.T) {
	scenario := Scenario{Name: "s", Clock: "14:00", Sessions: []Placement{{Ref: "tidepool/a", State: "working"}},
		Shots: []Shot{{Name: "a"}, {Name: "b", Keys: []string{"B"}}}}
	base := shotKey(scenario, scenario.Shots[0], "inputs-1")

	t.Run("when nothing has changed", func(t *testing.T) {
		t.Run("it should give the same key", func(t *testing.T) {
			assert.Equal(t, base, shotKey(scenario, scenario.Shots[0], "inputs-1"))
		})
	})

	t.Run("when another shot in the scenario changes", func(t *testing.T) {
		other := scenario
		other.Shots = []Shot{{Name: "a"}, {Name: "b", Keys: []string{"V"}}}

		t.Run("it should not refilm this one", func(t *testing.T) {
			assert.Equal(t, base, shotKey(other, other.Shots[0], "inputs-1"))
		})
	})

	for name, change := range map[string]func() string{
		"only the shot's title changes": func() string {
			return shotKey(scenario, Shot{Name: "a", Title: "A new title"}, "inputs-1")
		},
		"only the shot's description changes": func() string {
			return shotKey(scenario, Shot{Name: "a", Description: "New words for the same pixels."}, "inputs-1")
		},
		"only the scenario's description changes": func() string {
			changed := scenario
			changed.Description = "A city described afresh."
			return shotKey(changed, scenario.Shots[0], "inputs-1")
		},
	} {
		t.Run("when "+name, func(t *testing.T) {
			t.Run("it should keep the key, since the words draw nothing", func(t *testing.T) {
				assert.Equal(t, base, change())
			})
		})
	}

	for name, change := range map[string]func() string{
		"the shot itself changes": func() string {
			return shotKey(scenario, Shot{Name: "a", Keys: []string{"H"}}, "inputs-1")
		},
		"the city it films changes": func() string {
			changed := scenario
			changed.Clock = "23:00"
			return shotKey(changed, scenario.Shots[0], "inputs-1")
		},
		"the corpus or a binary changes": func() string {
			return shotKey(scenario, scenario.Shots[0], "inputs-2")
		},
	} {
		t.Run("when "+name, func(t *testing.T) {
			t.Run("it should give a new key", func(t *testing.T) {
				assert.NotEqual(t, base, change())
			})
		})
	}
}

func TestFilmCache(t *testing.T) {
	out := t.TempDir()
	media := []Media{{File: "s/a.png", Title: "Old", Description: "Old words."}, {File: "s/a.mp4", Title: "Old", Description: "Old words."}}
	for _, m := range media {
		writeFile(t, filepath.Join(out, m.File), "x")
	}
	require.NoError(t, remember(out, "s", "a", "key-1", media))
	shot := Shot{Name: "a", Title: "Old", Description: "Old words."}

	t.Run("when a shot's key matches and its files are there", func(t *testing.T) {
		got, ok := recall(out, "s", shot, "key-1")
		require.True(t, ok)

		t.Run("it should reuse the media", func(t *testing.T) {
			assert.Equal(t, media, got)
		})

		t.Run("and the shot's caption has changed since it was filmed", func(t *testing.T) {
			got, _ := recall(out, "s", Shot{Name: "a", Title: "New", Description: "New words."}, "key-1")

			t.Run("it should carry the shot's current title and description", func(t *testing.T) {
				assert.Equal(t, [][2]string{{"New", "New words."}, {"New", "New words."}}, captions(got))
			})
		})
	})

	t.Run("when the key differs", func(t *testing.T) {
		_, ok := recall(out, "s", shot, "key-2")

		t.Run("it should film it again", func(t *testing.T) {
			assert.False(t, ok)
		})
	})

	t.Run("when one of its files has gone", func(t *testing.T) {
		require.NoError(t, os.Remove(filepath.Join(out, "s/a.mp4")))
		_, ok := recall(out, "s", shot, "key-1")

		t.Run("it should film it again", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}

func TestPrune(t *testing.T) {
	out := t.TempDir()
	for _, f := range []string{"s/kept.png", "s/dropped.mp4", "gone/old.png", "manifest.json", "film.log", ".film-cache/s/a.json"} {
		writeFile(t, filepath.Join(out, f), "x")
	}
	require.NoError(t, prune(out, Manifest{Media: []Media{{File: "s/kept.png"}}}))
	exists := func(f string) bool {
		_, err := os.Stat(filepath.Join(out, f))
		return err == nil
	}

	t.Run("when the run is over", func(t *testing.T) {
		t.Run("it should keep what the manifest lists", func(t *testing.T) {
			assert.True(t, exists("s/kept.png"))
		})

		t.Run("it should remove media no shot made this time", func(t *testing.T) {
			assert.False(t, exists("s/dropped.mp4") || exists("gone/old.png"))
		})

		t.Run("it should keep the manifest, the log and the cache", func(t *testing.T) {
			assert.True(t, exists("manifest.json") && exists("film.log") && exists(".film-cache/s/a.json"))
		})
	})
}

func TestWritePublic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "manifest.json")
	require.NoError(t, writePublic(path, []byte("{}")))
	info, err := os.Stat(path)
	require.NoError(t, err)

	t.Run("when the manifest is written", func(t *testing.T) {
		t.Run("it should be readable by whatever serves or builds the site", func(t *testing.T) {
			assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
		})
	})
}

func TestInputsKey(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	writeFile(t, filepath.Join(corpus, "tidepool", "a.jsonl"), "{}\n")
	one, two := filepath.Join(dir, "one"), filepath.Join(dir, "two")
	writeFile(t, one, "a binary built here")
	writeFile(t, two, "the same source built elsewhere")
	key := func(t *testing.T, code string, binary string) string {
		t.Helper()
		k, err := inputsKey(corpus, code, binary)
		require.NoError(t, err)
		return k
	}

	t.Run("when the source is named", func(t *testing.T) {
		t.Run("it should key on it rather than on the binary, so another machine's build agrees", func(t *testing.T) {
			assert.Equal(t, key(t, "source-1", one), key(t, "source-1", two))
		})

		t.Run("and the source changes", func(t *testing.T) {
			t.Run("it should give a new key", func(t *testing.T) {
				assert.NotEqual(t, key(t, "source-1", one), key(t, "source-2", one))
			})
		})
	})

	t.Run("when no source is named", func(t *testing.T) {
		t.Run("it should fall back to the binary", func(t *testing.T) {
			assert.NotEqual(t, key(t, "", one), key(t, "", two))
		})
	})
}

func TestAdopt(t *testing.T) {
	out := t.TempDir()
	media := []Media{{File: "s/a.png", Title: "Old", Description: "Old words."}}
	writeFile(t, filepath.Join(out, "s/a.png"), "x")
	require.NoError(t, remember(out, "s", "a", "old-key", media))
	shot := Shot{Name: "a", Title: "New", Description: "New words."}

	t.Run("when a shot filmed under an old key is adopted", func(t *testing.T) {
		got, ok := adopt(out, "s", shot, "new-key")
		require.True(t, ok)
		_, now := recall(out, "s", shot, "new-key")

		t.Run("it should keep the media files it already has", func(t *testing.T) {
			assert.Equal(t, []string{"s/a.png"}, files(got))
		})

		t.Run("and the shot's caption has changed", func(t *testing.T) {
			t.Run("it should carry the shot's current title and description", func(t *testing.T) {
				assert.Equal(t, [][2]string{{"New", "New words."}}, captions(got))
			})
		})

		t.Run("it should answer to the new key from then on", func(t *testing.T) {
			assert.True(t, now)
		})
	})

	t.Run("when a shot was never filmed", func(t *testing.T) {
		_, ok := adopt(out, "s", Shot{Name: "never"}, "new-key")

		t.Run("it should have nothing to adopt", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}

func captions(media []Media) [][2]string {
	var out [][2]string
	for _, m := range media {
		out = append(out, [2]string{m.Title, m.Description})
	}
	return out
}

func files(media []Media) []string {
	var out []string
	for _, m := range media {
		out = append(out, m.File)
	}
	return out
}
