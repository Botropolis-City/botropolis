package targets

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const usage = `Usage: go run ./tools/acceptance-targets <run|map|changed|tier NAME> [flags]

  run       run every test but the performance tier, each in a process of
            its own, -p at a time (item 226)
  map       run every acceptance test on its own, under coverage, and record
            which packages each one executed, in the map
  changed   run only the tests that executed a package changed since the map
            was made; fall back to map, saying why, when the map cannot say
  tier      print the go test -run pattern for one tier's tests, behaviour
            or performance; fail if any test's marker disagrees with what it
            measures

Flags:
`

type options struct {
	pkg      string
	maps     string
	universe string
	parallel int
	timeout  time.Duration
}

func Main(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("acceptance-targets", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage); fs.PrintDefaults() }
	var o options
	fs.StringVar(&o.pkg, "pkg", "./testing/acceptance", "the acceptance test package")
	home, _ := os.UserHomeDir()
	fs.StringVar(&o.maps, "maps", MapDir(os.Getenv, home), "where the maps are kept, one per revision")
	fs.StringVar(&o.universe, "run", "", "only consider tests matching this pattern")
	fs.IntVar(&o.parallel, "p", DefaultParallel(runtime.NumCPU()), "how many tests to run or map at once")
	fs.DurationVar(&o.timeout, "timeout", 40*time.Minute, "each test's timeout, in run")
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fs.Usage()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	cmd, rest := args[0], args[1:]
	tierName := ""
	if cmd == "tier" {
		if len(rest) == 0 {
			fs.Usage()
			return 2
		}
		tierName, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch cmd {
	case "run":
		mod, err := module()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return RunPackage(o.pkg, mod+"/pkg/measure", o.universe, o.parallel, o.timeout, stdout)
	case "map":
		return mapAll(o, stdout, stderr)
	case "changed":
		return changed(o, stdout, stderr)
	case "tier":
		return printTier(o, tierName, stdout, stderr)
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

func printTier(o options, name string, stdout, stderr io.Writer) int {
	want, known := Behaviour, false
	for _, tier := range []Tier{Behaviour, Performance} {
		if tier.String() == name {
			want, known = tier, true
		}
	}
	if !known {
		fmt.Fprintf(stderr, "unknown tier %q: behaviour or performance\n", name)
		return 2
	}
	tests, err := readTiers(o)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, RunPattern(InTier(tests, want)))
	return 0
}

// readTiers reads the package's tiers and refuses any test whose marker
// disagrees with what it reaches, which the guard in the package also fails.
func readTiers(o options) (map[string]Test, error) {
	mod, err := module()
	if err != nil {
		return nil, err
	}
	tests, err := ReadTiers(o.pkg, mod+"/pkg/measure")
	if err != nil {
		return nil, err
	}
	var wrong []string
	for name, test := range tests {
		if test.Tier != test.Reaches {
			wrong = append(wrong, fmt.Sprintf("%s is marked %s but reaches %s", name, test.Tier, test.Reaches))
		}
	}
	if len(wrong) > 0 {
		sort.Strings(wrong)
		return nil, fmt.Errorf("tiers disagree: %s", strings.Join(wrong, "; "))
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
	cmd := exec.Command("go", "test", "-count=1", "-timeout", o.timeout.String(), "-cover", "-coverpkg="+coverPkg(mod), o.pkg, "-run", "^"+test+"$", "-args", "-test.gocoverdir="+dir)
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
	rev, err := git("rev-parse", "HEAD")
	if err != nil {
		fmt.Fprintln(stdout, "the map is not saved: git does not know HEAD")
		return 0
	}
	if dirty, err := git("status", "--porcelain"); err != nil || dirty != "" {
		fmt.Fprintf(stdout, "the map is not saved: the tree has changes %s does not, so it would be the wrong key\n", rev)
		return 0
	}
	m.Rev = rev
	if err := SaveMap(o.maps, &m); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "mapped %d tests to %s at %s\n", len(tests), o.maps, m.Rev)
	return 0
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// behindHEAD counts the commits from rev to HEAD, or -1 when rev is not an
// ancestor of HEAD or git does not know it.
func behindHEAD(rev string) int {
	if exec.Command("git", "merge-base", "--is-ancestor", rev, "HEAD").Run() != nil {
		return -1
	}
	out, err := git("rev-list", "--count", rev+"..HEAD")
	if err != nil {
		return -1
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return -1
	}
	return n
}

// acceptanceResolver traces a changed file in the acceptance package to the
// tests that reach what changed in it, from the diff since rev; a file git
// does not track yet changed everywhere.
func acceptanceResolver(o options, rev string) func(file string) ([]string, string) {
	diff, err := git("diff", "-U0", rev, "--", o.pkg)
	lines := ChangedLines(diff)
	return func(file string) ([]string, string) {
		if err != nil {
			return nil, "reading the diff of " + file + ": " + err.Error()
		}
		ranges, ok := lines[file]
		if !ok {
			ranges = []LineRange{{From: 1, To: 1 << 30}}
		}
		return ReachingChange(o.pkg, file, ranges)
	}
}

func changedSince(rev string) ([]string, error) {
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
	s := Situation{Tests: tests, CI: "unknown"}
	if common, err := git("rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		s.CI = ReadCI(common)
	}
	s.Map, s.Behind = NearestMap(o.maps, behindHEAD)
	var c Choice
	if s.Map != nil && s.Behind >= 0 {
		if s.Changed, err = changedSince(s.Map.Rev); err != nil {
			c = Choice{Fallback: err.Error()}
		}
	}
	if c.Fallback == "" {
		rev := ""
		if s.Map != nil {
			rev = s.Map.Rev
		}
		c = Choose(s, Resolver{
			PackageOf:  func(f string) (string, bool) { return PackageOf(".", mod, f) },
			Acceptance: acceptanceResolver(o, rev),
		})
	}
	switch {
	case c.Fallback != "":
		fmt.Fprintf(stdout, "full run: %s\n", c.Fallback)
		return mapAll(o, stdout, stderr)
	case len(c.Tests) == 0:
		fmt.Fprintln(stdout, c.Note)
		return 0
	}
	fmt.Fprintf(stdout, "selected, from the map at %s, %d commits behind: %s\n", s.Map.Rev, s.Behind, strings.Join(c.Tests, " "))
	return RunPackage(o.pkg, mod+"/pkg/measure", RunPattern(c.Tests), o.parallel, o.timeout, stdout)
}
