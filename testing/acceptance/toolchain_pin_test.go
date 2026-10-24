package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Item 103. The linter and Go are pinned in the repository, golangci-lint in
// tools/lint/go.mod and Go on go.mod's toolchain line, so make lint runs the
// same versions on every machine and in CI. make checks both, and this is the
// second way in, so the checks survive losing their line in the Makefile.
//
// The checks run in a make subprocess, which the test cache cannot see, so the
// files that decide their result are opened here to put them in its key.
func readPins(t *testing.T) {
	t.Helper()
	for _, f := range []string{"Makefile", "go.mod", "tools/lint/go.mod", "tools/check-pin"} {
		if _, err := os.ReadFile(filepath.Join("..", "..", f)); err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
	}
}

func pinnedVersion(t *testing.T, file, prefix string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", file))
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == prefix {
			return strings.TrimPrefix(strings.TrimPrefix(fields[1], "go"), "v")
		}
	}
	t.Fatalf("%s has no %s line", file, prefix)
	return ""
}

func decoy(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/usr/bin/env bash\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runMake(env []string, args ...string) (string, error) {
	cmd := exec.Command("make", append([]string{"-s", "-C", filepath.Join("..", "..")}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func withPath(dir string) string {
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func TestToolchainPins(t *testing.T) {
	readPins(t)
	lintPin := pinnedVersion(t, "tools/lint/go.mod", "github.com/golangci/golangci-lint/v2")
	goPin := pinnedVersion(t, "go.mod", "toolchain")
	const other = "9.9.9"

	t.Run("when the pinned linter is checked", func(t *testing.T) {
		lintDecoy := decoy(t, "golangci-lint", `echo "golangci-lint has version `+other+` built with go1.27.2"`+"\n")

		t.Run("and a different golangci-lint is first on PATH", func(t *testing.T) {
			out, err := runMake([]string{withPath(lintDecoy)}, "lint-version-check")

			t.Run("it should pass", func(t *testing.T) {
				assert.NoError(t, err, out)
			})
		})

		t.Run("and GOLANGCI_LINT is a linter reporting another version", func(t *testing.T) {
			out, err := runMake(nil, "lint-version-check", "GOLANGCI_LINT="+filepath.Join(lintDecoy, "golangci-lint"))

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, out)
			})

			t.Run("it should name the pinned version", func(t *testing.T) {
				assert.Contains(t, out, lintPin)
			})

			t.Run("it should name the version that ran", func(t *testing.T) {
				assert.Contains(t, out, other)
			})
		})
	})

	t.Run("when the pinned Go is checked", func(t *testing.T) {
		t.Run("and the toolchain run is the pin", func(t *testing.T) {
			out, err := runMake(nil, "go-version-check")

			t.Run("it should pass", func(t *testing.T) {
				assert.NoError(t, err, out)
			})
		})

		t.Run("and the machine's Go is another version", func(t *testing.T) {
			goDecoy := decoy(t, "go", `
if [[ "$GOTOOLCHAIN" == go* ]]; then echo "$GOTOOLCHAIN"; else echo "go`+other+`"; fi
`)
			out, err := runMake([]string{withPath(goDecoy), "GOTOOLCHAIN=auto"}, "go-version-check")

			t.Run("it should still run the pin", func(t *testing.T) {
				assert.NoError(t, err, out)
			})
		})

		t.Run("and GOTOOLCHAIN is a different version", func(t *testing.T) {
			local := localGoVersion(t)
			stale := ""
			for _, v := range []string{"1.27.1", "1.27.0", "1.26.0"} {
				if v != goPin && v != local {
					stale = v
					break
				}
			}
			gomod := filepath.Join(t.TempDir(), "go.mod")
			if err := os.WriteFile(gomod, []byte("module example.com/pin\n\ngo 1.27\n\ntoolchain go"+stale+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := runMake(nil, "go-version-check", "GO_MOD="+gomod, "GOTOOLCHAIN=local")

			t.Run("it should fail", func(t *testing.T) {
				assert.Error(t, err, out)
			})

			t.Run("it should name the pinned version", func(t *testing.T) {
				assert.Contains(t, out, stale)
			})

			t.Run("it should name the version that ran", func(t *testing.T) {
				assert.Contains(t, out, local)
			})
		})
	})
}

// Item 227: with GOTOOLCHAIN=local the machine's own Go runs, which need not
// be the pin.
func localGoVersion(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go env GOVERSION: %v", err)
	}
	version, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(string(out)), "go"), "-")
	return version
}
