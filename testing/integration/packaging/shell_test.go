package packaging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/shellinit"
)

// shell is one way a user loads the integration: the shell, how it runs a
// command line without reading any rc file, how it loads the script, and
// how it names the last exit status.
type shell struct {
	name   string
	binary string
	argv   []string
	load   string
	status string
}

var shells = []shell{
	{"bash", "bash", []string{"bash", "--norc", "--noprofile", "-c"}, `eval "$(botropolis shell-init bash)"`, "$?"},
	{"zsh", "zsh", []string{"zsh", "-f", "-c"}, `eval "$(botropolis shell-init zsh)"`, "$?"},
	{"fish", "fish", []string{"fish", "--no-config", "-c"}, "botropolis shell-init fish | source", "$status"},
	{"bash through the file the Linux packages install", "bash", []string{"bash", "--norc", "--noprofile", "-c"}, ". " + shim, "$?"},
}

var shim, _ = filepath.Abs("../../../packaging/botropolis.bash")

func TestShellIntegration(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script(1) is needed to give the wrapper a terminal")
	}

	for _, sh := range shells {
		t.Run("when the shell is "+sh.name, func(t *testing.T) {
			if _, err := exec.LookPath(sh.binary); err != nil {
				if os.Getenv("CI") != "" {
					t.Fatalf("%s is not installed, and CI must run every shell botropolis supports", sh.binary)
				}
				t.Skipf("%s is not installed", sh.binary)
			}

			for _, newExit := range []struct {
				name     string
				exit     string
				calls    string
				exitCode string
			}{
				{
					name:     "and botropolis starts the session",
					exit:     "0",
					calls:    "botropolis new DIR fix the queue\nclaude attach 16daefa1\n",
					exitCode: "0",
				},
				{
					name:     "and botropolis reports the directory as untrusted",
					exit:     "3",
					calls:    "botropolis new DIR fix the queue\nclaude fix the queue\n",
					exitCode: "0",
				},
				{
					name:     "and botropolis fails for any other reason",
					exit:     "1",
					calls:    "botropolis new DIR fix the queue\n",
					exitCode: "1",
				},
			} {
				t.Run(newExit.name, func(t *testing.T) {
					calls, exitCode := runWrapper(t, sh, newExit.exit, `claude fix the queue`)

					t.Run("it should run exactly these commands", func(t *testing.T) {
						assert.Equal(t, newExit.calls, calls)
					})

					t.Run("it should exit "+newExit.exitCode, func(t *testing.T) {
						assert.Equal(t, newExit.exitCode, exitCode)
					})
				})
			}

			t.Run("and a subcommand is run", func(t *testing.T) {
				calls, _ := runWrapper(t, sh, "0", `claude agents`)

				t.Run("it should go straight to the real claude", func(t *testing.T) {
					assert.Equal(t, "claude agents\n", calls)
				})
			})

			t.Run("and a flag is passed", func(t *testing.T) {
				calls, _ := runWrapper(t, sh, "0", `claude --resume`)

				t.Run("it should go straight to the real claude", func(t *testing.T) {
					assert.Equal(t, "claude --resume\n", calls)
				})
			})
		})
	}
}

func runWrapper(t *testing.T, sh shell, newExit, command string) (string, string) {
	t.Helper()
	bin, work, log := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "calls")

	scripts := t.TempDir()
	for _, name := range shellinit.Shells {
		script, err := shellinit.Script(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(scripts, name), []byte(script), 0o600))
	}

	fake := "#!/bin/sh\necho \"$(basename \"$0\") $*\" | sed \"s|$PWD|DIR|\" >> \"$CALLS\"\n"
	writeExecutable(t, filepath.Join(bin, "claude"), fake)
	writeExecutable(t, filepath.Join(bin, "botropolis"),
		"#!/bin/sh\nif [ \"$1\" = shell-init ]; then cat \"$SCRIPTS/$2\"; exit; fi\n"+
			strings.TrimPrefix(fake, "#!/bin/sh\n")+"echo 16daefa1\nexit \"$NEW_EXIT\"\n")

	inner := sh.load + "; " + command + "; echo " + sh.status + " > \"$CALLS.exit\""
	argv := append(append([]string{}, sh.argv...), inner)
	cmd := exec.Command("script", "-qec", shellQuote(argv), "/dev/null")
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("script", append([]string{"-q", "/dev/null"}, argv...)...)
	}
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CALLS="+log,
		"SCRIPTS="+scripts,
		"NEW_EXIT="+newExit,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	calls, _ := os.ReadFile(log)
	exitCode, err := os.ReadFile(log + ".exit")
	require.NoError(t, err, string(out))
	return string(calls), strings.TrimSpace(string(exitCode))
}

func shellQuote(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}
