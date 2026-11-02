// Command settle waits for a quiet machine before a performance gate, from
// inside the landing lock (item 330), so a landing cannot run between the
// check and the reading.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const usage = `settle — wait for a quiet machine before a performance gate

Usage:
  settle [--one N] [--five N] [--limit D] [--poll D]
  settle --help

Waits until the 1-minute load average is under --one and the 5-minute under
--five (the method's bars, 3 and 4), looking every --poll, and says how long
it waited and at what load. After --limit it says the machine never went
quiet and exits 0, so the run goes on and its figures read as a busy
machine's. With CI set it does not wait and only says the load. It exits 1
when /proc/loadavg cannot be read.
`

type load struct{ One, Five, Fifteen float64 }

func (l load) String() string { return fmt.Sprintf("%.2f %.2f %.2f", l.One, l.Five, l.Fifteen) }

type quietBars struct{ One, Five float64 }

func (b quietBars) String() string {
	return fmt.Sprintf("the bars %g over 1 minute and %g over 5", b.One, b.Five)
}

func (b quietBars) quiet(l load) bool { return l.One < b.One && l.Five < b.Five }

type settleOptions struct {
	Read        func() (load, error)
	Sleep       func(time.Duration)
	Since       func() time.Duration
	Bars        quietBars
	Poll, Limit time.Duration
	CI          bool
}

func parseLoadavg(s string) (load, error) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return load{}, fmt.Errorf("/proc/loadavg reads %q, not three averages", s)
	}
	var v [3]float64
	for i := range v {
		n, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return load{}, fmt.Errorf("/proc/loadavg reads %q: %w", s, err)
		}
		v[i] = n
	}
	return load{v[0], v[1], v[2]}, nil
}

func settle(o settleOptions) (string, error) {
	l, err := o.Read()
	if err != nil {
		return "", err
	}
	if o.CI {
		return fmt.Sprintf("not waiting for a quiet machine: CI is set, and a shared runner reports rather than gates (item 305); load %v", l), nil
	}
	for !o.Bars.quiet(l) {
		if o.Since() >= o.Limit {
			return fmt.Sprintf("not quiet after %v: load %v, %v; measuring anyway, so read this run's figures as a busy machine's", o.Since(), l, o.Bars), nil
		}
		o.Sleep(o.Poll)
		if l, err = o.Read(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("quiet after %v: load %v, %v", o.Since().Round(time.Second), l, o.Bars), nil
}

func main() {
	fs := flag.NewFlagSet("settle", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	one := fs.Float64("one", 3, "the 1-minute load's bar")
	five := fs.Float64("five", 4, "the 5-minute load's bar")
	limit := fs.Duration("limit", 30*time.Minute, "how long to wait")
	poll := fs.Duration("poll", 10*time.Second, "how often to look")
	for _, a := range os.Args[1:] {
		if a == "--help" || a == "-h" {
			fmt.Print(usage)
			return
		}
	}
	_ = fs.Parse(os.Args[1:])
	start := time.Now()
	said, err := settle(settleOptions{
		Read: func() (load, error) {
			b, err := os.ReadFile("/proc/loadavg")
			if err != nil {
				return load{}, err
			}
			return parseLoadavg(string(b))
		},
		Sleep: time.Sleep,
		Since: func() time.Duration { return time.Since(start) },
		Bars:  quietBars{*one, *five},
		Poll:  *poll, Limit: *limit,
		CI: os.Getenv("CI") != "",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "settle: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("settle: " + said)
}
