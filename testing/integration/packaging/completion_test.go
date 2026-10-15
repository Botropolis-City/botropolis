package packaging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completion is how one shell loads botropolis's completion script and
// asks it what `botropolis shell-` completes to, through the real binary.
type completion struct {
	shell  string
	script string
	argv   []string
	ask    string
	want   string
}

var completions = []completion{
	{"bash", "bash", []string{"bash", "--norc", "--noprofile", "-c"},
		`source "$BASH_COMPLETION"; source "$SCRIPT"; COMP_WORDS=(botropolis shell-); COMP_CWORD=1; COMP_LINE="botropolis shell-"; COMP_POINT=${#COMP_LINE}; ` +
			`$(complete -p botropolis | awk '{print $(NF-1)}') botropolis shell- botropolis; printf '%s\n' "${COMPREPLY[@]}"`, "shell-init"},
	{"zsh", "zsh", []string{"zsh", "-f", "-c"},
		`fpath=("$(dirname "$SCRIPT")" $fpath); autoload -U compinit; compinit -u -d "$TMPDIR/zcompdump"; print -r -- $_comps[botropolis]`, "_botropolis"},
	{"fish", "fish", []string{"fish", "--no-config", "-c"},
		`source $SCRIPT; complete -C 'botropolis shell-'`, "shell-init"},
}

// bashCompletion finds the bash-completion package's loader. Cobra's bash
// script is built on it, and the directory packages install into is only
// read through it, so any bash that completes anything has it loaded.
func bashCompletion(t *testing.T) string {
	t.Helper()
	candidates := []string{"/usr/share/bash-completion/bash_completion"}
	if prefix, err := exec.Command("brew", "--prefix").Output(); err == nil {
		p := strings.TrimSpace(string(prefix))
		candidates = append(candidates, p+"/etc/profile.d/bash_completion.sh", p+"/etc/bash_completion", p+"/share/bash-completion/bash_completion")
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("bash-completion is not installed (looked in %s), and CI must test bash completion", strings.Join(candidates, ", "))
	}
	return "/dev/null"
}

func TestCompletionScripts(t *testing.T) {
	bin := t.TempDir()
	build := exec.Command("go", "build", "-buildvcs=false", "-o", filepath.Join(bin, "botropolis"), "../../../cmd/botropolis")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	for _, c := range completions {
		t.Run("when the shell is "+c.shell, func(t *testing.T) {
			if _, err := exec.LookPath(c.shell); err != nil {
				if os.Getenv("CI") != "" {
					t.Fatalf("%s is not installed, and CI must run every shell botropolis supports", c.shell)
				}
				t.Skipf("%s is not installed", c.shell)
			}
			dir := t.TempDir()
			name := map[string]string{"bash": "botropolis.bash", "zsh": "_botropolis", "fish": "botropolis.fish"}[c.shell]
			script, err := exec.Command(filepath.Join(bin, "botropolis"), "completion", c.shell).Output()
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), script, 0o600))

			cmd := exec.Command(c.argv[0], append(c.argv[1:], c.ask)...)
			cmd.Env = append(os.Environ(), "BASH_COMPLETION="+bashCompletion(t),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"SCRIPT="+filepath.Join(dir, name),
				"TMPDIR="+t.TempDir(),
			)
			got, err := cmd.CombinedOutput()
			require.NoError(t, err, string(got))

			t.Run("and botropolis shell- is completed", func(t *testing.T) {
				t.Run("it should offer "+c.want, func(t *testing.T) {
					assert.Contains(t, strings.Fields(string(got)), c.want, string(got))
				})
			})
		})
	}
}
