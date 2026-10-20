package acceptance_test

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"botropolis.city/botropolis/pkg/city"
	"botropolis.city/botropolis/pkg/claude"
	"botropolis.city/botropolis/pkg/commands"
	"botropolis.city/botropolis/pkg/config"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
)

// update rewrites the surface golden files from this tree:
// go test ./testing/acceptance -run TestSurface -update
var update = flag.Bool("update", false, "rewrite testdata/surface from this tree")

const surfaceDir = "testdata/surface"

// A surface is one thing 1.0.0 promises, as a set of lines, each a name
// and then, after a tab, what about it is promised. tools/check-version
// compares these files with the ones at the last release (item 149).
type surface struct {
	name  string
	lines func(t *testing.T) []string
}

func TestSurface(t *testing.T) {
	surfaces := []surface{
		{"config", configSurface},
		{"bar", barSurface},
		{"status-json", statusSurface},
		{"data", dataSurface},
		{"install", installSurface},
	}
	for _, s := range surfaces {
		t.Run("when the "+s.name+" surface is read from this tree", func(t *testing.T) {
			got := setOf(s.lines(t))
			path := filepath.Join(surfaceDir, s.name+".txt")
			if *update {
				require.NoError(t, os.MkdirAll(surfaceDir, 0o755))
				require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
			}
			want, err := os.ReadFile(path)

			t.Run("it should equal "+path+" (rewrite it with -update in the same commit)", func(t *testing.T) {
				require.NoError(t, err)
				assert.Equal(t, string(want), got)
			})
		})
	}

	t.Run("when the config keys are read", func(t *testing.T) {
		keys := lineNames(configSurface(t))

		t.Run("it should not offer projection, removed before 1.0", func(t *testing.T) {
			assert.NotContains(t, keys, "projection")
		})
	})

	t.Run("when each config key with a closed set of values is read", func(t *testing.T) {
		parsers := map[string]func(string) bool{
			config.KeyLiveliness:   func(v string) bool { _, ok := city.ParseLiveliness(v); return ok },
			config.KeyDetail:       func(v string) bool { _, ok := city.ParseDetail(v); return ok },
			config.KeyDistrictType: func(v string) bool { _, ok := city.ParseTyping(v); return ok },
			config.KeyUsageRefresh: func(v string) bool { _, err := config.ParseUsageRefresh(v); return err == nil },
		}
		for key, values := range config.Values {
			parse, ok := parsers[key]
			t.Run("and it is "+key, func(t *testing.T) {
				t.Run("it should have a parser this test knows", func(t *testing.T) {
					assert.True(t, ok)
				})
				if !ok {
					return
				}
				for _, v := range values {
					t.Run("it should accept "+v, func(t *testing.T) {
						assert.True(t, parse(v))
					})
				}
				t.Run("it should refuse a value it does not list", func(t *testing.T) {
					assert.False(t, parse("no-such-value"))
				})
			})
		}
	})

	t.Run("when status --json writes a session", func(t *testing.T) {
		var rendered strings.Builder
		require.NoError(t, commands.RenderJSON(&rendered, state.Snapshot{Sessions: []state.Session{sampleSession()}}))
		whole, err := json.Marshal([]state.Session{sampleSession()})
		require.NoError(t, err)

		t.Run("it should still emit every key beyond the promised ones", func(t *testing.T) {
			assert.JSONEq(t, string(whole), rendered.String())
		})
	})

	t.Run("when a config file still sets projection", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(file, []byte("projection = \"top\"\n"), 0o600))
		v := config.NewViper()
		v.Set(config.KeyConfig, file)
		_, err := config.New(v)

		t.Run("it should still load", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	for _, f := range dataFixtures(t) {
		t.Run("when this build reads "+f.kind+" version "+f.version+", from "+f.path, func(t *testing.T) {
			t.Run("it should keep what it holds", func(t *testing.T) {
				assert.NoError(t, f.check(t))
			})
		})
	}
}

func setOf(lines []string) string {
	seen := map[string]bool{}
	var out []string
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n") + "\n"
}

func lineNames(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		name, _, _ := strings.Cut(l, "\t")
		out = append(out, name)
	}
	return out
}

// configSurface is every key config.toml reads, its type, and each value
// a key with a closed set accepts. Defaults are not promised: the key
// bindings' defaults are the city's to change (Aria, 2026-10-20).
func configSurface(t *testing.T) []string {
	v := config.NewViper()
	var lines []string
	for _, key := range v.AllKeys() {
		lines = append(lines, fmt.Sprintf("%s\t%T", key, v.Get(key)))
	}
	for key, values := range config.Values {
		for _, value := range values {
			lines = append(lines, key+"="+value)
		}
	}
	return lines
}

// barSurface is the keys of bar's JSON and every class it emits, which
// status bars style by.
func barSurface(t *testing.T) []string {
	var lines []string
	snapshots := map[string]state.Snapshot{"nothing live": {}}
	for _, s := range state.Order {
		snapshots[string(s)] = state.Snapshot{Sessions: []state.Session{{ID: "a", Title: "a", State: s}}}
	}
	for _, snapshot := range snapshots {
		line := commands.Summarize(snapshot)
		data, err := json.Marshal(line)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(data, &fields))
		for key, value := range fields {
			lines = append(lines, key+"\t"+jsonType(value))
		}
		lines = append(lines, "class="+line.Class)
	}
	return lines
}

// sampleSession has every promised key set, so none is left out as
// empty.
func sampleSession() state.Session {
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	return state.Session{
		ID: fixtureSession, PID: 4242, Harness: "claude", Title: "Fix the CI queue", State: state.Working,
		CWD: fixtureRoot, Branch: "main", Model: "claude-opus-5", ContextPercent: 12.5,
		StartedAt: at, LastActivity: at,
		PRs: []claude.PR{{Number: 1, URL: "https://example.com/pr/1", Repository: "w/lib", State: "open"}},
	}
}

// statusSurface is each promised key of a `status --json` object, its
// JSON type, the keys of a pull request in prs, and every state.
func statusSurface(t *testing.T) []string {
	var buf strings.Builder
	require.NoError(t, commands.RenderJSON(&buf, state.Snapshot{Sessions: []state.Session{sampleSession()}}))
	var objects []map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &objects))
	require.Len(t, objects, 1)
	var lines []string
	for _, key := range commands.PromisedStatusKeys {
		value, ok := objects[0][key]
		require.True(t, ok, "status --json does not emit the promised key %s", key)
		lines = append(lines, key+"\t"+jsonType(value))
	}
	prs, _ := objects[0]["prs"].([]any)
	require.Len(t, prs, 1)
	for key, value := range prs[0].(map[string]any) {
		lines = append(lines, "prs[]."+key+"\t"+jsonType(value))
	}
	for _, s := range state.Order {
		lines = append(lines, "state="+string(s))
	}
	return lines
}

func jsonType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case []any:
		return "array"
	}
	return "object"
}

// A dataFixture is a file an earlier Botropolis wrote, which every later
// 1.x must read without losing what it holds.
type dataFixture struct {
	kind, version, path string
	check               func(t *testing.T) error
}

const dataDir = "testdata/data"

func dataFixtures(t *testing.T) []dataFixture {
	entries, err := os.ReadDir(dataDir)
	require.NoError(t, err)
	var out []dataFixture
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		kind, version, ok := strings.Cut(base, "-")
		require.True(t, ok, "%s is not <kind>-<version>.<ext>", e.Name())
		path := filepath.Join(dataDir, e.Name())
		out = append(out, dataFixture{kind: kind, version: version, path: path, check: checkFor(kind, path)})
	}
	return out
}

func dataSurface(t *testing.T) []string {
	var lines []string
	for _, f := range dataFixtures(t) {
		lines = append(lines, f.kind+" version "+f.version)
	}
	return lines
}

// Every fixture holds the same thing: a session at /w/lib, in slot 4 of
// its district where it has slots, and one message where it has usage.
const (
	fixtureSession = "aaaaaaaa-0000-0000-0000-000000000149"
	fixtureRoot    = "/w/lib"
)

func checkFor(kind, path string) func(t *testing.T) error {
	switch kind {
	case "layout":
		return func(t *testing.T) error {
			layout, err := city.LoadLayout(copied(t, path))
			if err != nil {
				return err
			}
			if d := layout.Districts[fixtureRoot]; d == nil || d.Slots[fixtureSession] != 4 {
				return fmt.Errorf("%s lost the session's slot", path)
			}
			return nil
		}
	case "ledger":
		return func(t *testing.T) error {
			ledger, err := state.LoadLedger(copied(t, path))
			if err != nil {
				return err
			}
			if ledger[fixtureSession].CWD != fixtureRoot {
				return fmt.Errorf("%s lost the session", path)
			}
			return nil
		}
	case "stats":
		return func(t *testing.T) error {
			file := copied(t, path)
			store, err := stats.Open(file)
			if err != nil {
				return err
			}
			if err := store.Close(); err != nil {
				return err
			}
			db, err := sql.Open("sqlite", file)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			var input int
			if err := db.QueryRow("SELECT input FROM messages m JOIN message_sessions s ON s.message_id = m.id WHERE s.session_id = ?", fixtureSession).Scan(&input); err != nil {
				return fmt.Errorf("%s lost the message: %w", path, err)
			}
			if input != 10 {
				return fmt.Errorf("%s: input is %d, not 10", path, input)
			}
			return nil
		}
	}
	return func(t *testing.T) error { return fmt.Errorf("no check for %s", kind) }
}

// copied is a fixture copied somewhere a migration can write, so the
// committed file is never changed by reading it.
func copied(t *testing.T, path string) string {
	t.Helper()
	src, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = src.Close() }()
	dst := filepath.Join(t.TempDir(), filepath.Base(path))
	out, err := os.Create(dst)
	require.NoError(t, err)
	_, err = io.Copy(out, src)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	return dst
}

// installSurface is what install commands, package managers and Claude
// Code's settings name: the package, the formula, every fixed release
// asset, the systemd units and the hook command.
func installSurface(t *testing.T) []string {
	data, err := os.ReadFile("../../packaging/nfpm.yaml")
	require.NoError(t, err)
	var nfpm struct {
		Name     string `yaml:"name"`
		Contents []struct {
			Dst string `yaml:"dst"`
		} `yaml:"contents"`
	}
	require.NoError(t, yaml.Unmarshal(data, &nfpm))
	lines := []string{nfpm.Name + "\tpackage"}
	for _, c := range nfpm.Contents {
		if dir, unit := filepath.Split(c.Dst); dir == "/usr/lib/systemd/user/" {
			lines = append(lines, unit+"\tunit")
		}
	}
	formulas, err := filepath.Glob("../../packaging/brew/*.rb.in")
	require.NoError(t, err)
	for _, f := range formulas {
		lines = append(lines, strings.TrimSuffix(filepath.Base(f), ".rb.in")+"\tformula")
	}
	versioned := []string{
		"botropolis_0.1.5_amd64.deb", "botropolis_0.1.5_arm64.deb",
		"botropolis-0.1.5-1.x86_64.rpm", "botropolis-0.1.5-1.aarch64.rpm",
		"botropolis-0.1.5-1-x86_64.pkg.tar.zst", "botropolis-0.1.5-1-aarch64.pkg.tar.zst",
		"botropolis-0.1.5-linux-amd64.tar.gz", "botropolis-0.1.5-linux-arm64.tar.gz",
	}
	dir, out, err := runAliases(t, "0.1.5", versioned...)
	require.NoError(t, err, string(out))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	given := map[string]bool{}
	for _, v := range versioned {
		given[v] = true
	}
	for _, e := range entries {
		if !given[e.Name()] {
			lines = append(lines, e.Name()+"\tasset")
		}
	}
	lines = append(lines, config.NewViper().GetString(config.KeyHookCommand)+"\thook")
	return lines
}
