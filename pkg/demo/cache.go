package demo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cacheDir is where a film run remembers what each shot was made from.
const cacheDir = ".film-cache"

// shotKey is what a shot was made from: the shot, the city it films --
// its scenario without the other shots, which a change to one must not
// refilm the rest for -- and the inputs every shot shares, the corpus
// and the binaries. The same key means the same pixels, so the words
// that caption them are left out: they draw nothing, and recall and
// adopt bring them up to date (item 235).
func shotKey(scenario Scenario, shot Shot, inputs string) string {
	city := scenario
	city.Shots = nil
	city.Description = ""
	shot.Title, shot.Description = "", ""
	data, _ := json.Marshal(struct {
		City   Scenario
		Shot   Shot
		Inputs string
	}{city, shot, inputs})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// inputsKey hashes what every shot of a run is made from: each file in
// the corpus, by path and content, and the code that stages and draws --
// named by a hash of its source when one is given, which another
// machine's build of the same commit agrees with, and by the binaries'
// own bytes when not, which it does not.
func inputsKey(corpus, source string, binaries ...string) (string, error) {
	h := sha256.New()
	if source != "" {
		_, _ = io.WriteString(h, "source\x00"+source+"\x00")
		binaries = nil
	}
	var paths []string
	err := filepath.WalkDir(corpus, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, p := range append(paths, binaries...) {
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, p+"\x00")
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type cached struct {
	Key   string  `json:"key"`
	Media []Media `json:"media"`
}

func cachePath(out, scenario, shot string) string {
	return filepath.Join(out, cacheDir, scenario, shot+".json")
}

// recall is a shot's media from an earlier run, if it was made from the
// same key and every file it made is still there.
func recall(out, scenario string, shot Shot, key string) ([]Media, bool) {
	c, ok := cachedShot(out, scenario, shot.Name)
	if !ok || c.Key != key {
		return nil, false
	}
	return captioned(c.Media, shot), true
}

// captioned is a shot's media under the shot's caption as it now reads.
func captioned(media []Media, shot Shot) []Media {
	out := make([]Media, len(media))
	for i, m := range media {
		m.Title, m.Description = shot.Title, shot.Description
		out[i] = m
	}
	return out
}

// adopt takes a shot's media from an earlier run whatever key it was
// made under, and files it under this one: for when the key changed but
// the picture cannot have, such as the key's own recipe changing. It is
// a claim only a person can make, so only --adopt makes it.
func adopt(out, scenario string, shot Shot, key string) ([]Media, bool) {
	c, ok := cachedShot(out, scenario, shot.Name)
	if !ok {
		return nil, false
	}
	media := captioned(c.Media, shot)
	if remember(out, scenario, shot.Name, key, media) != nil {
		return nil, false
	}
	return media, true
}

// cachedShot is what the cache holds for a shot, if all its files are
// still there.
func cachedShot(out, scenario, shot string) (cached, bool) {
	data, err := os.ReadFile(cachePath(out, scenario, shot))
	if err != nil {
		return cached{}, false
	}
	var c cached
	if json.Unmarshal(data, &c) != nil || len(c.Media) == 0 {
		return cached{}, false
	}
	for _, m := range c.Media {
		for _, f := range []string{m.File, m.Poster} {
			if f == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(out, f)); err != nil {
				return cached{}, false
			}
		}
	}
	return c, true
}

// remember records what a shot made, and from what.
func remember(out, scenario, shot, key string, media []Media) error {
	data, err := json.MarshalIndent(cached{Key: key, Media: media}, "", "  ")
	if err != nil {
		return err
	}
	return writeBytes(cachePath(out, scenario, shot), append(data, '\n'))
}

// prune removes media no shot made or reused this run -- a shot renamed
// or dropped from a scenario -- and leaves the manifest, the log and the
// cache, which are not media.
func prune(out string, manifest Manifest) error {
	keep := map[string]bool{}
	for _, m := range manifest.Media {
		keep[filepath.FromSlash(m.File)] = true
		if m.Poster != "" {
			keep[filepath.FromSlash(m.Poster)] = true
		}
	}
	var stale []string
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(out, p)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && rel != "." {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.Contains(rel, string(filepath.Separator)) || keep[rel] {
			return nil
		}
		stale = append(stale, p)
		return nil
	})
	if err != nil {
		return err
	}
	var errs []error
	for _, p := range stale {
		errs = append(errs, os.Remove(p))
	}
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			_ = os.Remove(filepath.Join(out, e.Name()))
		}
	}
	return errors.Join(errs...)
}

// writePublic writes a file meant for others to read -- the manifest the
// site is built from -- where writeBytes keeps a staged home private.
func writePublic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
