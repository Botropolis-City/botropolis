package targets

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const usage = `Usage: go run ./tools/acceptance-targets <map|changed> [flags]

  map       run every acceptance test on its own, under coverage, and record
            which packages each one executed, in the map
  changed   run only the tests that executed a package changed since the map
            was made; fall back to map, saying why, when the map cannot say

Flags:
`

type options struct {
	pkg      string
	mapFile  string
	universe string
	parallel int
}

func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("acceptance-targets", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage); fs.PrintDefaults() }
	var o options
	fs.StringVar(&o.pkg, "pkg", "./testing/acceptance", "the acceptance test package")
	fs.StringVar(&o.mapFile, "map", filepath.Join(".cache", "acceptance-map.json"), "where the map is kept")
	fs.StringVar(&o.universe, "run", "", "only consider tests matching this pattern")
	fs.IntVar(&o.parallel, "p", 4, "how many tests to map at once")
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fs.Usage()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	cmd := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch cmd {
	case "map":
		return mapAll(o, stdout, stderr)
	case "changed":
		return changed(o, stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown command %q\n", cmd)
	fs.Usage()
	return 2
}

func module() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			return f[1], nil
		}
	}
	return "", errors.New("go.mod names no module")
}

func listTests(o options) ([]string, error) {
	out, err := exec.Command("go", "test", "-list", ".", o.pkg).Output()
	if err != nil {
		return nil, fmt.Errorf("listing the tests: %w", err)
	}
	var match *regexp.Regexp
	if o.universe != "" {
		if match, err = regexp.Compile(o.universe); err != nil {
			return nil, err
		}
	}
	var tests []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Test") && (match == nil || match.MatchString(line)) {
			tests = append(tests, line)
		}
	}
	return tests, nil
}

func coverPkg(mod string) string {
	return mod + "/pkg/...," + mod + "/cmd/..."
}

func mapOne(o options, mod, test string) ([]string, []byte, error) {
	dir, err := os.MkdirTemp("", "acceptance-cover-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	cmd := exec.Command("go", "test", "-count=1", "-cover", "-coverpkg="+coverPkg(mod), o.pkg, "-run", "^"+test+"$", "-args", "-test.gocoverdir="+dir)
	cmd.Env = append(os.Environ(), "BOTROPOLIS_ACCEPTANCE_COVER="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, out, err
	}
	profile := filepath.Join(dir, "profile.txt")
	if b, err := exec.Command("go", "tool", "covdata", "textfmt", "-i", dir, "-o", profile).CombinedOutput(); err != nil {
		return nil, b, err
	}
	data, err := os.ReadFile(profile)
	if err != nil {
		return nil, nil, err
	}
	return TouchedPackages(string(data), mod), out, nil
}

func mapAll(o options, stdout, stderr io.Writer) int {
	mod, err := module()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tests, err := listTests(o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	m := Map{Tests: map[string][]string{}}
	var mu sync.Mutex
	failed := 0
	work := make(chan string)
	var wg sync.WaitGroup
	for range max(o.parallel, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for test := range work {
				pkgs, out, err := mapOne(o, mod, test)
				mu.Lock()
				if err != nil {
					failed++
					fmt.Fprintf(stdout, "FAIL %s\n%s\n", test, out)
				} else {
					m.Tests[test] = pkgs
					fmt.Fprintf(stdout, "ok   %s (%d packages)\n", test, len(pkgs))
				}
				mu.Unlock()
			}
		}()
	}
	for _, test := range tests {
		work <- test
	}
	close(work)
	wg.Wait()
	if failed > 0 {
		fmt.Fprintf(stdout, "%d of %d tests failed; the map is not updated\n", failed, len(tests))
		return 1
	}
	if rev, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		m.Rev = strings.TrimSpace(string(rev))
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(o.mapFile), 0o755)
	}
	if err == nil {
		err = os.WriteFile(o.mapFile, append(data, '\n'), 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "mapped %d tests to %s at %s\n", len(tests), o.mapFile, m.Rev)
	return 0
}

func loadMap(file string) *Map {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var m Map
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return &m
}

func changedSince(rev string) ([]string, error) {
	if rev == "" || exec.Command("git", "cat-file", "-e", rev+"^{commit}").Run() != nil {
		return nil, fmt.Errorf("the map's revision %q is unknown to git", rev)
	}
	seen := map[string]bool{}
	var files []string
	for _, args := range [][]string{{"diff", "--name-only", rev}, {"ls-files", "--others", "--exclude-standard"}} {
		out, err := exec.Command("git", args...).Output()
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			if f := sc.Text(); f != "" && !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}

func changed(o options, stdout, stderr io.Writer) int {
	mod, err := module()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tests, err := listTests(o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	m := loadMap(o.mapFile)
	var c Choice
	if m == nil {
		c = Choose(nil, tests, nil, nil)
	} else if files, err := changedSince(m.Rev); err != nil {
		c = Choice{Fallback: err.Error()}
	} else {
		c = Choose(m, tests, files, func(f string) (string, bool) { return PackageOf(".", mod, f) })
	}
	switch {
	case c.Fallback != "":
		fmt.Fprintf(stdout, "full run: %s\n", c.Fallback)
		return mapAll(o, stdout, stderr)
	case len(c.Tests) == 0:
		fmt.Fprintln(stdout, c.Note)
		return 0
	}
	fmt.Fprintf(stdout, "selected: %s\n", strings.Join(c.Tests, " "))
	run := exec.Command("go", "test", o.pkg, "-run", "^("+strings.Join(c.Tests, "|")+")$")
	run.Stdout, run.Stderr = stdout, stderr
	if err := run.Run(); err != nil {
		return 1
	}
	return 0
}
