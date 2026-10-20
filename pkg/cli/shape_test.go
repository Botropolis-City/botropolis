package cli_test

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/config"
)

// helpSections reads a help page into its headed sections, each a list
// of the first word on every indented line under the heading.
func helpSections(help string) map[string][]string {
	sections := map[string][]string{}
	heading := ""
	for _, line := range strings.Split(help, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			heading = ""
		case !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":"):
			heading = line
		case heading != "" && strings.HasPrefix(line, " "):
			sections[heading] = append(sections[heading], strings.Fields(line)[0])
		}
	}
	return sections
}

// flagNames are the long names listed in a help page's Flags section.
func flagNames(help string) []string {
	_, after, _ := strings.Cut(help, "Flags:\n")
	block, _, _ := strings.Cut(after, "\n\n")
	var names []string
	for _, m := range regexp.MustCompile(`(?m)^\s+(?:-\w, )?(--[a-z][a-z-]*)`).FindAllStringSubmatch(block, -1) {
		names = append(names, m[1])
	}
	slices.Sort(names)
	return names
}

func TestRootHelp(t *testing.T) {
	h := newHarness(&config.Config{})
	require.NoError(t, h.run("--help"))
	help := h.out.String()
	sections := helpSections(help)

	for _, c := range []struct{ group, command string }{
		{"Sessions:", "new"},
		{"Sessions:", "attach"},
		{"Sessions:", "stop"},
		{"Sessions:", "rm"},
		{"Sessions:", "status"},
		{"Sessions:", "find"},
		{"The city:", "city"},
		{"Setting up and checking:", "setup"},
		{"Setting up and checking:", "doctor"},
		{"Setting up and checking:", "events"},
		{"For other programs:", "bar"},
		{"For other programs:", "notify"},
		{"For other programs:", "shell-init"},
	} {
		t.Run("it should list "+c.command+" under "+c.group, func(t *testing.T) {
			assert.Contains(t, sections[c.group], c.command)
		})
	}

	for _, old := range []string{"tui", "resume", "prune", "install-hooks", "version", "completion"} {
		t.Run("it should not list "+old, func(t *testing.T) {
			assert.NotRegexp(t, regexp.MustCompile(`(?m)^  `+old+` `), help)
		})
	}

	t.Run("it should list no flag but --config, --set, --home, --socket, --help and --version", func(t *testing.T) {
		assert.Equal(t, []string{"--config", "--help", "--home", "--set", "--socket", "--version"}, flagNames(help))
	})
}

func TestDeprecationNotes(t *testing.T) {
	t.Run("when an old per-key flag is passed", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status", "--labels=false"))

		t.Run("it should print one line on stderr naming --set", func(t *testing.T) {
			assert.Equal(t, "botropolis: --labels is deprecated and goes at 1.0: use --set labels=false\n", h.errOut.String())
		})
	})

	for _, c := range []struct {
		old      []string
		now      string
		hidden   func(h *harness) bool
		happened func(h *harness) any
		want     any
	}{
		{[]string{"tui"}, "status --live", commandHidden("tui"),
			func(h *harness) any { return h.tui.runs }, 1},
		{[]string{"--tui"}, "status --live", func(h *harness) bool { return h.root.Flags().Lookup("tui").Hidden },
			func(h *harness) any { return h.tui.runs }, 1},
		{[]string{"resume", "--dir", "/tmp/x", "08b1d655-276f-495b-a61f-2214af35a97a"}, "attach", commandHidden("resume"),
			func(h *harness) any { return h.sessions.calls }, []string{"open 08b1d655-276f-495b-a61f-2214af35a97a dir=/tmp/x here=false"}},
		{[]string{"prune", "--older-than", "48h", "--dry-run"}, "rm --older-than", commandHidden("prune"),
			func(h *harness) any { return h.sessions.calls }, []string{"prune 48h0m0s dry=true"}},
		{[]string{"install-hooks", "--remove"}, "setup", commandHidden("install-hooks"),
			func(h *harness) any { return h.hooks.calls }, []string{"remove"}},
		{[]string{"version"}, "--version", commandHidden("version"),
			func(h *harness) any { return h.out.String() }, "botropolis dev\n"},
	} {
		name := strings.Join(c.old, " ")
		t.Run("when "+name+" is run", func(t *testing.T) {
			h := newHarness(&config.Config{})
			require.NoError(t, h.run(c.old...))

			t.Run("it should do what "+c.now+" does", func(t *testing.T) {
				assert.Equal(t, c.want, c.happened(h))
			})

			t.Run("it should print one stderr line naming "+c.now, func(t *testing.T) {
				assert.Equal(t, "botropolis: "+c.old[0]+" is deprecated and goes at 1.0: use botropolis "+c.now+"\n", h.errOut.String())
			})

			t.Run("it should be hidden from help", func(t *testing.T) {
				assert.True(t, c.hidden(h))
			})
		})
	}
}

func commandHidden(name string) func(h *harness) bool {
	return func(h *harness) bool {
		for _, c := range h.root.Commands() {
			if c.Name() == name {
				return c.Hidden
			}
		}
		return false
	}
}

func TestCompletionCommand(t *testing.T) {
	h := newHarness(&config.Config{})
	err := h.run("completion", "bash")

	t.Run("it should succeed", func(t *testing.T) {
		assert.NoError(t, err)
	})

	t.Run("it should print a script", func(t *testing.T) {
		assert.Contains(t, h.out.String(), "botropolis")
	})

	t.Run("it should print nothing on stderr", func(t *testing.T) {
		assert.Empty(t, h.errOut.String())
	})
}

func TestAttachCLI(t *testing.T) {
	for _, c := range []struct {
		scenario string
		tty      bool
		args     []string
		want     string
	}{
		{"when attach is run from a terminal", true, []string{"attach", "0898d7e4"}, "open 0898d7e4 dir= here=true"},
		{"when attach is run from no terminal", false, []string{"attach", "0898d7e4"}, "open 0898d7e4 dir= here=false"},
		{"when attach is run from a terminal with --new-window", true, []string{"attach", "--new-window", "0898d7e4"}, "open 0898d7e4 dir= here=false"},
		{"when attach is given --dir", true, []string{"attach", "--dir", "/tmp/x", "0898d7e4"}, "open 0898d7e4 dir=/tmp/x here=true"},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			h := newHarness(&config.Config{})
			h.tty = c.tty
			require.NoError(t, h.run(c.args...))

			t.Run("it should open the session so", func(t *testing.T) {
				assert.Equal(t, []string{c.want}, h.sessions.calls)
			})
		})
	}

	t.Run("when the runner fails", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.sessions.err = errors.New("no session 0898d7e4")

		t.Run("it should return its error", func(t *testing.T) {
			assert.EqualError(t, h.run("attach", "0898d7e4"), "no session 0898d7e4")
		})
	})

	t.Run("when attach is given no id", func(t *testing.T) {
		t.Run("it should fail", func(t *testing.T) {
			assert.Error(t, newHarness(&config.Config{}).run("attach"))
		})
	})
}

func TestRmCLI(t *testing.T) {
	t.Run("when rm is given ids", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("rm", "0898d7e4", "16daefa1"))

		t.Run("it should remove each, in order", func(t *testing.T) {
			assert.Equal(t, []string{"rm 0898d7e4", "rm 16daefa1"}, h.sessions.calls)
		})
	})

	t.Run("when rm is given ids and --dry-run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("rm", "--dry-run", "0898d7e4"))

		t.Run("it should remove nothing", func(t *testing.T) {
			assert.Empty(t, h.sessions.calls)
		})

		t.Run("it should say what it would remove", func(t *testing.T) {
			assert.Equal(t, "would remove 0898d7e4\n", h.out.String())
		})
	})

	t.Run("when rm is given --older-than", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("rm", "--older-than", "48h"))

		t.Run("it should prune with that cutoff", func(t *testing.T) {
			assert.Equal(t, []string{"prune 48h0m0s dry=false"}, h.sessions.calls)
		})

		t.Run("it should print each one removed", func(t *testing.T) {
			assert.Contains(t, h.out.String(), "removed 11111111  old job")
		})
	})

	t.Run("when rm is given --older-than and --dry-run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("rm", "--older-than", "48h", "--dry-run"))

		t.Run("it should print what it would remove", func(t *testing.T) {
			assert.Contains(t, h.out.String(), "would remove 11111111  old job")
		})
	})

	t.Run("when rm is given both ids and --older-than", func(t *testing.T) {
		h := newHarness(&config.Config{})

		t.Run("it should fail saying one or the other", func(t *testing.T) {
			assert.EqualError(t, h.run("rm", "--older-than", "48h", "0898d7e4"), "rm takes ids or --older-than, not both")
		})
	})
}

func TestStatusLive(t *testing.T) {
	t.Run("when status is run with --live", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status", "--live"))

		t.Run("it should run the live table", func(t *testing.T) {
			assert.Equal(t, 1, h.tui.runs)
		})

		t.Run("it should print no rows", func(t *testing.T) {
			assert.Empty(t, h.out.String())
		})
	})

	t.Run("when status is run with --live and --json", func(t *testing.T) {
		h := newHarness(&config.Config{})

		t.Run("it should fail saying they do not combine", func(t *testing.T) {
			assert.EqualError(t, h.run("status", "--live", "--json"), "--live and --json do not combine")
		})
	})

	t.Run("when status's help is printed", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("status", "--help"))

		t.Run("it should describe --json as a plain flag giving an array", func(t *testing.T) {
			assert.Regexp(t, `--json\s+a JSON array, one object per session`, h.out.String())
		})
	})
}

func TestSetupCLI(t *testing.T) {
	t.Run("when setup is run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.shell = "/usr/bin/zsh"
		require.NoError(t, h.run("setup"))

		t.Run("it should install the hooks", func(t *testing.T) {
			assert.Equal(t, []string{"install"}, h.hooks.calls)
		})

		t.Run("it should make sure the daemon runs", func(t *testing.T) {
			assert.Equal(t, []string{"ensure dry=false"}, h.daemon.calls)
		})

		t.Run("it should print the rc line for $SHELL", func(t *testing.T) {
			assert.Contains(t, h.out.String(), `add to ~/.zshrc: eval "$(botropolis shell-init zsh)"`)
		})
	})

	t.Run("when $SHELL is not a shell botropolis knows", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.shell = "/bin/tcsh"
		require.NoError(t, h.run("setup"))

		for _, line := range []string{
			`add to ~/.bashrc: eval "$(botropolis shell-init bash)"`,
			`add to ~/.zshrc: eval "$(botropolis shell-init zsh)"`,
			`add to ~/.config/fish/config.fish: botropolis shell-init fish | source`,
		} {
			t.Run("it should print "+line, func(t *testing.T) {
				assert.Contains(t, h.out.String(), line)
			})
		}
	})

	t.Run("when setup is run with --dry-run", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("setup", "--dry-run"))

		t.Run("it should only print the hooks", func(t *testing.T) {
			assert.Equal(t, []string{"dry-run"}, h.hooks.calls)
		})

		t.Run("it should ask the daemon for a dry run", func(t *testing.T) {
			assert.Equal(t, []string{"ensure dry=true"}, h.daemon.calls)
		})
	})

	t.Run("when setup is run with --remove", func(t *testing.T) {
		h := newHarness(&config.Config{})
		require.NoError(t, h.run("setup", "--remove"))

		t.Run("it should remove the hooks", func(t *testing.T) {
			assert.Equal(t, []string{"remove"}, h.hooks.calls)
		})

		t.Run("it should start nothing", func(t *testing.T) {
			assert.Empty(t, h.daemon.calls)
		})
	})

	t.Run("when starting the daemon fails", func(t *testing.T) {
		h := newHarness(&config.Config{})
		h.daemon.err = errors.New("systemctl --user enable --now botropolisd: exit status 1")
		err := h.run("setup")

		t.Run("it should return the command and its error", func(t *testing.T) {
			assert.EqualError(t, err, "systemctl --user enable --now botropolisd: exit status 1")
		})

		t.Run("it should leave the hooks installed", func(t *testing.T) {
			assert.Equal(t, []string{"install"}, h.hooks.calls)
		})
	})
}
