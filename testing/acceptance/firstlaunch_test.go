//go:build linux

package acceptance_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/proto"
	"botropolis.city/botropolis/pkg/state"
	"botropolis.city/botropolis/pkg/stats"
	"botropolis.city/botropolis/testing/helpers"
)

const firstLaunchGB = "BOTROPOLIS_FIRST_LAUNCH_GB"

var percentIn = regexp.MustCompile(`(\d+)%`)

// Item 159: 0.2.2's first launch took up to four cores, and 0.2.3's a
// peak of 414%. The limits are the budget's (performance/budgets.md,
// First launch): at most one core in any 1 s sample, at background
// priority, and at most 256 MiB of Private_Dirty at every sample. Both
// are per sample, so the 256 MiB history the suite builds measures the
// same rate a large one would, only for less time; set
// BOTROPOLIS_FIRST_LAUNCH_GB=3 to run it at the item's size for a
// report.
func TestFirstLaunch(t *testing.T) {
	history := firstLaunchHistory()

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over %d MiB of history", history>>20), func(t *testing.T) {
		l := newLaunch(t, history, 8<<20)
		run := l.start(t)
		run.untilRead(t, l.written)
		read := bytesRead(t, run.pid)

		t.Run("it should read the whole history", func(t *testing.T) {
			assert.GreaterOrEqual(t, read, l.written)
		})
	})

	// Item 226: on a loaded machine the daemon, at background priority,
	// can go seconds without the CPU partway through its read, and a wait
	// that took that for idle read 80 MiB of 256. Stopping it is the same
	// starvation, made certain.
	t.Run("when the daemon is held still for 4 s partway through its first launch, as a starved one is", func(t *testing.T) {
		l := newLaunch(t, 64<<20, 8<<20)
		run := l.start(t)
		require.Eventually(t, func() bool { return bytesRead(t, run.pid) >= 8<<20 }, time.Minute, 20*time.Millisecond)
		require.NoError(t, run.cmd.Process.Signal(syscall.SIGSTOP))
		resumed := make(chan struct{})
		go func() {
			defer close(resumed)
			time.Sleep(4 * time.Second)
			_ = run.cmd.Process.Signal(syscall.SIGCONT)
		}()
		run.untilRead(t, l.written)
		read := bytesRead(t, run.pid)
		<-resumed

		t.Run("it should still read the whole history before the wait ends", func(t *testing.T) {
			assert.GreaterOrEqual(t, read, l.written)
		})
	})
}

// TestFirstLaunchCost is TestFirstLaunch's limits on a core and on
// Private_Dirty, which need a quiet machine, so they run in the
// performance tier (item 226).
func TestFirstLaunchCost(t *testing.T) {
	performanceTier(t)
	const (
		maxCores   = 1.0
		dirtyLimit = 256 << 20
	)
	history := firstLaunchHistory()

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over %d MiB of history", history>>20), func(t *testing.T) {
		l := newLaunch(t, history, 8<<20)
		run := l.start(t)
		run.untilIdleMeasured(t)
		t.Logf("%d MiB of history in %s: peak %.0f%% of a core, peak %d MiB Private_Dirty",
			l.written>>20, run.took.Round(time.Second), run.peakShare*100, run.peakDirty>>20)

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, run.peakShare, maxCores)
		})

		t.Run("it should stay under 256 MiB of Private_Dirty at every sample", func(t *testing.T) {
			assert.Less(t, run.peakDirty, int64(dirtyLimit))
		})
	})
}

// firstLaunchHistory is the history a first launch reads: 256 MiB, or
// BOTROPOLIS_FIRST_LAUNCH_GB for a report.
func firstLaunchHistory() int64 {
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		return int64(gb * (1 << 30))
	}
	return 256 << 20
}

// Item 203: on Aria's history, 3.3 GiB in 195 transcripts, the 30 that
// were live or active in the last day held 708 MiB, and the daemon's
// first load read them all, whole or from a day back, at a core for 9 s
// before the map could draw. The suite's home had none, so its first
// load was instant. This home has the real one's shape: several hundred
// stopped sessions, and a day's work held mostly in one very large live
// session (Aria's largest that day was 482 MiB, the next 116 MiB).
//
// The first snapshot is held to what drawing it needs, each
// transcript's window, in bytes, which load cannot move: 52 MiB here,
// against 434 MiB before the fix. Its work is logged but gates only at
// a real size, BOTROPOLIS_FIRST_LAUNCH_GB set for a report, against 2
// CPU-seconds, the 5 s budget less the city's own second. In the suite
// no limit in CPU-seconds separates the fix from what it fixed: the
// same fixed first load measured 0.66-0.87 CPU-seconds at a load of 9
// and up to 1.65 at a load of 30, and the code before the fix
// 1.50-2.13, so any limit between them would fail landings at random.
// Every 1 s sample is held to one core, which the code before the fix
// broke at 110-118%, and the unpaced first load at 111-115%.
func TestFirstLaunchLoad(t *testing.T) {
	const (
		window = 2 * 64 << 10
		slack  = 16 << 20
	)
	shape := newRealShape()

	t.Run(shape.when(), func(t *testing.T) {
		load := shape.load(t, (*daemonRun).untilRead, nil)

		t.Run("it should read no more than each transcript's window before its first snapshot", func(t *testing.T) {
			assert.LessOrEqual(t, load.readFirst, load.transcripts*window+slack)
		})

		t.Run("and it has settled", func(t *testing.T) {
			for _, sid := range load.live {
				t.Run("it should hold live session "+sid[:8]+"'s usage whole", func(t *testing.T) {
					assert.False(t, sessionIn(t, load.settled, sid).Usage.Partial)
				})
			}

			t.Run("it should give the plant the day's whole output", func(t *testing.T) {
				assert.Equal(t, int64(5*load.turns), load.settled.Power.Output)
			})
		})
	})
}

// TestFirstLaunchLoadCost is TestFirstLaunchLoad's limits in CPU, which
// need a quiet machine, so they run in the performance tier (item 226).
func TestFirstLaunchLoadCost(t *testing.T) {
	performanceTier(t)
	const (
		maxCPUSeconds = 2.0
		clockTicksSec = 100
		maxCores      = 1.0
	)
	shape := newRealShape()

	t.Run(shape.when(), func(t *testing.T) {
		cpuFirst := -1.0
		load := shape.load(t, func(r *daemonRun, t *testing.T, _ int64) { r.untilIdleMeasured(t) }, func(pid int) { cpuFirst = float64(cpuTicks(t, pid)) / clockTicksSec })
		t.Logf("first snapshot after %.2f CPU-seconds; idle after %s at a peak of %.0f%% of a core",
			cpuFirst, load.run.took.Round(time.Second), load.run.peakShare*100)

		if shape.scale != 1 {
			t.Run("it should send its first snapshot within 2 CPU-seconds", func(t *testing.T) {
				assert.LessOrEqual(t, cpuFirst, maxCPUSeconds)
			})
		}

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, load.run.peakShare, maxCores)
		})
	})
}

// realShape is a home shaped like a real one: several hundred stopped
// sessions, and a day's work held mostly in one very large live session.
type realShape struct {
	scale                 float64
	stopped               int
	old                   int64
	liveSizes, todaySizes []int64
}

func newRealShape() realShape {
	scale := 1.0
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		scale = gb * 1024 / 484
	}
	sized := func(n int64) int64 { return int64(float64(n) * scale) }
	return realShape{
		scale:      scale,
		stopped:    400,
		old:        sized(256 << 10),
		liveSizes:  []int64{sized(288 << 20), sized(16 << 20)},
		todaySizes: []int64{sized(64 << 20), sized(16 << 20)},
	}
}

func (s realShape) when() string {
	return fmt.Sprintf("when the daemon starts on an empty state directory over a home shaped like a real one: %d stopped sessions of %d KiB, live sessions of %d and %d MiB, and stopped ones active today of %d and %d MiB", s.stopped, s.old>>10, s.liveSizes[0]>>20, s.liveSizes[1]>>20, s.todaySizes[0]>>20, s.todaySizes[1]>>20)
}

type firstLoad struct {
	run         *daemonRun
	readFirst   int64
	transcripts int64
	live        []string
	turns       int
	settled     state.Snapshot
}

// load starts a daemon on the shape, reads its first snapshot and, once
// settled has returned, the settled one. settled is handed the daemon and
// how many bytes of history the shape wrote. first, when not nil, is
// called with the daemon's pid as the first snapshot arrives.
func (s realShape) load(t *testing.T, settled func(r *daemonRun, t *testing.T, written int64), first func(pid int)) firstLoad {
	t.Helper()
	root := t.TempDir()
	home := helpers.HomeAt(t, filepath.Join(root, "home"))
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	var written int64
	for i := range s.stopped {
		w, _ := syntheticSession(t, home, syntheticSID(i), at.Add(time.Duration(i)*time.Hour), s.old)
		written += w
	}
	var load firstLoad
	for i, pid := range []int{os.Getpid(), os.Getppid()} {
		sid := syntheticSID(s.stopped + i)
		home.Session(pid, sid, syntheticCWD, "interactive", "idle")
		w, n := syntheticSession(t, home, sid, now.Add(-3*time.Hour), s.liveSizes[i])
		load.live, load.turns, written = append(load.live, sid), load.turns+n, written+w
	}
	for i, size := range s.todaySizes {
		w, n := syntheticSession(t, home, syntheticSID(s.stopped+2+i), now.Add(-20*time.Hour), size)
		load.turns, written = load.turns+n, written+w
	}
	l := &launch{root: root, home: home.Path}
	load.run = l.start(t)
	quiet := make(chan struct{})
	go func() {
		defer close(quiet)
		settled(load.run, t, written)
	}()
	var client *proto.Client
	require.Eventually(t, func() bool {
		c, err := proto.Dial(load.run.socket)
		client = c
		return err == nil
	}, time.Minute, 10*time.Millisecond)
	_, err := client.Snapshot()
	require.NoError(t, err)
	load.readFirst = bytesRead(t, load.run.pid)
	if first != nil {
		first(load.run.pid)
	}
	_ = client.Close()
	<-quiet
	client, err = proto.Dial(load.run.socket)
	require.NoError(t, err)
	load.settled, err = client.Snapshot()
	require.NoError(t, err)
	_ = client.Close()
	load.transcripts = int64(s.stopped + 4)
	t.Logf("first snapshot after %d MiB", load.readFirst>>20)
	return load
}

func sessionIn(t *testing.T, snapshot state.Snapshot, id string) state.Session {
	t.Helper()
	for _, s := range snapshot.Sessions {
		if s.ID == id {
			return s
		}
	}
	require.FailNow(t, "no session "+id)
	return state.Session{}
}

// Item 159: on the 265U, 0.2.2 stopped 6 s into a first launch read
// 89% of the history again when started on the same state. The
// transcripts here are longer than a checkpoint, so a stop lands inside
// one; the daemon is killed rather than asked to quit, so nothing but
// the checkpoints can save it.
func TestFirstLaunchResumes(t *testing.T) {
	const (
		history    = 96 << 20
		perSession = 48 << 20
		stopAt     = 40 << 20
	)

	t.Run("when the daemon is killed part-way through its first launch and started again on the same state", func(t *testing.T) {
		whole := newLaunch(t, history, perSession)
		uninterrupted := whole.start(t)
		uninterrupted.untilRead(t, whole.written)
		once := bytesRead(t, uninterrupted.pid)
		again := once - whole.written

		l := newLaunch(t, history, perSession)
		first := l.start(t)
		require.Eventually(t, func() bool { return bytesRead(t, first.pid) >= stopAt }, 5*time.Minute, 50*time.Millisecond)
		first.kill(t)
		before := first.read
		second := l.start(t)
		second.untilRead(t, l.written-before)
		total := before + bytesRead(t, second.pid)
		t.Logf("one launch read %d MiB of %d; killed at %d MiB and started again, %d MiB in all", once>>20, whole.written>>20, before>>20, total>>20)

		t.Run("it should read no more than one launch, one checkpoint, and a second start's own reads", func(t *testing.T) {
			assert.LessOrEqual(t, total, once+stats.CheckpointEvery+again)
		})
	})
}

// Item 159: the budget's "a map within 5 s of opening the city, with
// what is known so far and a percentage for the rest". The city is the
// debug build, whose frame log says when each frame is drawn, whether it
// drew the city, and what the footer's reading line said on it.
//
// Wall-clock seconds cannot gate the suite: on a shared machine at a
// load of 40, a city on an empty home took 12-15 s to draw, so they are
// asserted only with BOTROPOLIS_FIRST_LAUNCH_TIMED=1, for a report. What
// gates is work: the CPU-seconds the city spends before it draws the
// city, which load barely moves: 0.14-0.29 CPU-seconds on 2026-10-21.
// The limit is 1 CPU-second, about 3.5 times that, and inside the 5 s
// budget; slice 3 set it at 5, which Aria's note pointed out could only
// catch a fifteenfold regression. BOTROPOLIS_FIRST_LAUNCH_FRAME=path
// keeps the last frame, for a person to judge.
func TestFirstLaunchCity(t *testing.T) {
	helpers.NeedXvfb(t)
	const within = 5 * time.Second
	timed := os.Getenv("BOTROPOLIS_FIRST_LAUNCH_TIMED") != ""

	t.Run("when the city opens on a daemon partway through a first launch", func(t *testing.T) {
		run := openFirstLaunchCity(t, nil)

		t.Run("it should show a percentage", func(t *testing.T) {
			require.NotEmpty(t, run.shown)
			assert.Contains(t, run.shown[0], "%")
		})

		t.Run("it should show it on every frame until the read is done", func(t *testing.T) {
			assert.NotContains(t, run.shown, "")
		})

		t.Run("it should move as the read goes on", func(t *testing.T) {
			require.NotEmpty(t, run.percents)
			assert.Less(t, run.percents[0], run.percents[len(run.percents)-1])
		})

		t.Run("it should never go backwards", func(t *testing.T) {
			assert.True(t, slices.IsSorted(run.percents))
		})

		if timed {
			t.Run("it should draw the city within 5 s", func(t *testing.T) {
				assert.True(t, run.firstDrawn > 0 && run.firstDrawn <= within)
			})

			t.Run("it should show the percentage within 5 s", func(t *testing.T) {
				assert.True(t, run.firstReading > 0 && run.firstReading <= within)
			})
		}
	})
}

// TestFirstLaunchCityCPU is TestFirstLaunchCity's limit in CPU-seconds,
// which needs a quiet machine, so it runs in the performance tier (item
// 226) while the percentage it checks runs on every landing.
func TestFirstLaunchCityCPU(t *testing.T) {
	performanceTier(t)
	helpers.NeedXvfb(t)
	const (
		maxCPUSeconds = 1.0
		clockTicksSec = 100
	)

	t.Run("when the city opens on a daemon partway through a first launch", func(t *testing.T) {
		cpuToDrawn := -1.0
		openFirstLaunchCity(t, func(pid int) { cpuToDrawn = float64(cpuTicks(t, pid)) / clockTicksSec })
		t.Logf("first city drawn after %.2f CPU-seconds", cpuToDrawn)

		t.Run("it should spend at most 1 CPU-second before it draws the city", func(t *testing.T) {
			assert.True(t, cpuToDrawn >= 0 && cpuToDrawn <= maxCPUSeconds)
		})
	})
}

type firstLaunchCity struct {
	shown                    []string
	percents                 []int
	firstDrawn, firstReading time.Duration
}

// openFirstLaunchCity films a debug city opened on a daemon partway
// through a first launch, and reads what its footer said on each frame.
// drawn, when not nil, is called with the city's pid as it first draws the
// city.
func openFirstLaunchCity(t *testing.T, drawn func(pid int)) firstLaunchCity {
	t.Helper()
	bin, err := debugCity()
	require.NoError(t, err)
	keep := os.Getenv("BOTROPOLIS_FIRST_LAUNCH_FRAME")
	history := int64(256 << 20)
	if keep != "" {
		history = 1 << 30
	}
	l := newLaunch(t, history, 8<<20)
	daemon := l.start(t)
	require.Eventually(t, func() bool { return bytesRead(t, daemon.pid) >= 64<<20 }, time.Minute, 50*time.Millisecond)
	frames := filepath.Join(t.TempDir(), "frames")
	require.NoError(t, os.MkdirAll(frames, 0o755))
	city := exec.Command(bin, "city", "--headless", "--record", frames, "--seconds", "8", "--fps", "2", "--window", "1600x900", "--socket", daemon.socket)
	// The read is what this films, so the recording opens on the
	// first snapshot rather than waiting for the read (item 202).
	city.Env = append(os.Environ(), "HOME="+l.home, "XDG_STATE_HOME="+filepath.Join(l.root, "state"), "BOTROPOLIS_FRAME_OPEN_UNREAD=1")
	out, err := city.StdoutPipe()
	require.NoError(t, err)
	var stderr syncBuffer
	city.Stderr = &stderr
	start := time.Now()
	require.NoError(t, city.Start())
	t.Cleanup(func() { _ = city.Process.Kill(); _ = city.Wait() })
	var run firstLaunchCity
	var log strings.Builder
	lines := bufio.NewScanner(out)
	lines.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for lines.Scan() {
		line := lines.Text()
		log.WriteString(line + "\n")
		if strings.HasPrefix(line, "still: ") && run.firstDrawn == 0 {
			run.firstDrawn = time.Since(start)
			if drawn != nil {
				drawn(city.Process.Pid)
			}
		}
		if strings.HasPrefix(line, "reading: ") && run.firstReading == 0 {
			run.firstReading = time.Since(start)
		}
	}
	_ = city.Wait()
	for _, f := range helpers.ParseFrameLog(strings.NewReader(log.String())) {
		if f.Reading != "" || len(run.shown) > 0 {
			run.shown = append(run.shown, f.Reading)
		}
	}
	for len(run.shown) > 0 && run.shown[len(run.shown)-1] == "" {
		run.shown = run.shown[:len(run.shown)-1]
	}
	for _, line := range run.shown {
		if m := percentIn.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			run.percents = append(run.percents, n)
		}
	}
	t.Logf("first city drawn at %s; first reading line at %s; %d frames from it", run.firstDrawn.Round(time.Millisecond), run.firstReading.Round(time.Millisecond), len(run.shown))
	if len(run.shown) == 0 {
		t.Logf("the city wrote: %s", stderr.String())
	}
	if keep != "" {
		pngs, err := filepath.Glob(filepath.Join(frames, "*.png"))
		require.NoError(t, err)
		require.NotEmpty(t, pngs)
		data, err := os.ReadFile(pngs[len(pngs)-1])
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(keep, data, 0o644))
	}
	return run
}

// Item 159: a city opened while the daemon's first load runs waited on
// an empty window. The daemon here is a stand-in that answers as this
// build, says it has read 40 of 120 sessions, and never sends the first
// snapshot, so the city waits for as long as the test watches.
func TestFirstLaunchCityWaiting(t *testing.T) {
	helpers.NeedXvfb(t)
	const want = "Reading your sessions · 40 of 120"

	t.Run("when the city opens while the daemon is 40 sessions of 120 into its first load", func(t *testing.T) {
		bin, err := debugCity()
		require.NoError(t, err)
		sock := shortSocket(t)
		listener, err := net.Listen("unix", sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close() })
		go holdingDaemon(listener, state.LoadProgress{Done: 40, Total: 120})
		root := t.TempDir()
		frames := filepath.Join(root, "frames")
		require.NoError(t, os.MkdirAll(frames, 0o755))
		// A recording counts its seconds from the city opening, which it
		// never does here, so the city is watched for a while and stopped.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		city := exec.CommandContext(ctx, bin, "city", "--headless", "--record", frames, "--seconds", "3", "--fps", "2", "--window", "1600x900", "--socket", sock)
		city.Env = append(os.Environ(), "HOME="+filepath.Join(root, "home"), "XDG_STATE_HOME="+filepath.Join(root, "state"))
		// BOTROPOLIS_FIRST_LAUNCH_OPENING_FRAME=path keeps a frame of the
		// panel, for a person to judge.
		keep := os.Getenv("BOTROPOLIS_FIRST_LAUNCH_OPENING_FRAME")
		if keep != "" {
			city.Env = append(city.Env, "BOTROPOLIS_OPENING_FRAME="+keep)
		}
		var out syncBuffer
		city.Stdout = &out
		// The city's own Xvfb holds its output open, so the whole group
		// is stopped, and the wait gives up on the pipe soon after.
		city.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		city.Cancel = func() error { return syscall.Kill(-city.Process.Pid, syscall.SIGKILL) }
		city.WaitDelay = 3 * time.Second
		_ = city.Run()
		openings, drawn := 0, 0
		for _, line := range strings.Split(out.String(), "\n") {
			switch {
			case line == "opening: "+want:
				openings++
			case strings.HasPrefix(line, "still: "):
				drawn++
			}
		}
		t.Logf("%d frames said %q, %d drew the city", openings, want, drawn)
		if keep != "" {
			_, err := os.Stat(keep)
			require.NoError(t, err, "the city kept no frame of the opening panel")
		}

		t.Run("it should say how far the daemon has got", func(t *testing.T) {
			assert.Positive(t, openings)
		})

		t.Run("it should draw no city while it waits", func(t *testing.T) {
			assert.Zero(t, drawn)
		})
	})
}

// holdingDaemon answers every connection as a daemon of this build
// holding its first load would: a loading request with load, a
// subscription with nothing, ever.
func holdingDaemon(listener net.Listener, load state.LoadProgress) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer func() { _ = conn.Close() }()
			var request proto.Request
			if err := proto.NewDecoder(conn).Decode(&request); err != nil {
				return
			}
			switch request.Op {
			case proto.OpLoading:
				_ = proto.Write(conn, proto.Response{Peer: proto.Self(), Loading: &load})
			case proto.OpSubscribe:
				_, _ = io.Copy(io.Discard, conn)
			default:
				_ = proto.Write(conn, proto.Response{Peer: proto.Self()})
			}
		}()
	}
}

// launch is a home of synthetic history with its own state directory.
type launch struct {
	root, home string
	written    int64
}

func newLaunch(t *testing.T, history, perSession int64) *launch {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	return &launch{root: root, home: home, written: syntheticHistory(t, helpers.HomeAt(t, home), history, perSession)}
}

func (l *launch) env() []string {
	return append(os.Environ(), "HOME="+l.home, "XDG_STATE_HOME="+filepath.Join(l.root, "state"), "PATH=/nonexistent")
}

// daemonRun is one daemon started on a launch, and what it was sampled
// at.
type daemonRun struct {
	socket    string
	cmd       *exec.Cmd
	pid       int
	read      int64
	peakShare float64
	peakDirty int64
	took      time.Duration
	killed    bool
}

func (l *launch) start(t *testing.T) *daemonRun {
	t.Helper()
	bin, err := botropolisd()
	require.NoError(t, err)
	sock := shortSocket(t)
	cmd := exec.Command(bin, "--socket", sock, "--config", filepath.Join(l.root, "config.toml"))
	cmd.Env = l.env()
	require.NoError(t, cmd.Start())
	r := &daemonRun{cmd: cmd, pid: cmd.Process.Pid, socket: sock}
	t.Cleanup(func() {
		if !r.killed {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return r
}

// kill stops the daemon at once, noting what it had read.
func (r *daemonRun) kill(t *testing.T) {
	t.Helper()
	r.read = bytesRead(t, r.pid)
	require.NoError(t, r.cmd.Process.Kill())
	_ = r.cmd.Wait()
	r.killed = true
}

// untilRead waits until the daemon has read at least want bytes, then
// until its reads hold still for three samples a second apart, or ten
// minutes. It waits on the read itself, not on the CPU (item 226): a
// daemon starved of the CPU on a loaded machine looked idle to that,
// with 80 MiB of 256 read.
func (r *daemonRun) untilRead(t *testing.T, want int64) {
	t.Helper()
	const (
		stillSamples = 3
		gaveUp       = 10 * time.Minute
	)
	start := time.Now()
	still, last := 0, int64(-1)
	for still < stillSamples && time.Since(start) < gaveUp {
		time.Sleep(time.Second)
		n := bytesRead(t, r.pid)
		switch {
		case n < want:
			still = 0
		case n == last:
			still++
		default:
			still = 0
		}
		last = n
	}
	r.took = time.Since(start)
}

// untilQuiet samples the daemon each second until it has used under 5%
// of a core for three samples running, handing each sample to sampled.
func (r *daemonRun) untilQuiet(t *testing.T, sampled func(share float64)) {
	t.Helper()
	const (
		clockTicksSec = 100
		idleShare     = 0.05
		idleSamples   = 3
		gaveUp        = 10 * time.Minute
	)
	ticks := func() int64 { user, system := procTicks(t, r.pid); return user + system }
	quiet := 0
	start := time.Now()
	last, at := ticks(), time.Now()
	for quiet < idleSamples && time.Since(start) < gaveUp {
		time.Sleep(time.Second)
		n, now := ticks(), time.Now()
		share := float64(n-last) / clockTicksSec / now.Sub(at).Seconds()
		last, at = n, now
		if sampled != nil {
			sampled(share)
		}
		if share < idleShare {
			quiet++
		} else {
			quiet = 0
		}
	}
	r.took = time.Since(start)
}

// procTicks reads a process's user and system time, in clock ticks, from
// /proc/<pid>/stat. Waits use it directly; a reading a test asserts on goes
// through meters_test.go, which is how the performance tier is found.
func procTicks(t *testing.T, pid int) (user, system int64) {
	t.Helper()
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	require.NoError(t, err)
	s := string(stat)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	user, err = strconv.ParseInt(fields[11], 10, 64)
	require.NoError(t, err)
	system, err = strconv.ParseInt(fields[12], 10, 64)
	require.NoError(t, err)
	return user, system
}

// syntheticHistory writes stopped sessions under home until they hold
// size bytes, each at most perSession, each turn a prompt carrying a
// tool's worth of output and a reply with usage, the shape that makes
// real transcripts large. It returns how many bytes it wrote.
func syntheticHistory(t *testing.T, home *helpers.Home, size, perSession int64) int64 {
	t.Helper()
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var written int64
	for s := 0; written < size; s++ {
		n, _ := syntheticSession(t, home, syntheticSID(s), at.Add(time.Duration(s)*time.Hour), min(perSession, size-written))
		written += n
	}
	return written
}

const syntheticCWD = "/home/user/src/history"

func syntheticSID(s int) string {
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", s, s)
}

// syntheticSession writes one session of turns a second apart from
// start until it holds size bytes, and returns the bytes and turns.
func syntheticSession(t *testing.T, home *helpers.Home, sid string, start time.Time, size int64) (int64, int) {
	t.Helper()
	output := strings.Repeat("x", 6<<10)
	path := home.Transcript(sid, syntheticCWD)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	w := bufio.NewWriterSize(f, 1<<20)
	var written int64
	turns := 0
	for ; written < size; turns++ {
		ts := start.Add(time.Duration(turns) * time.Second).Format(time.RFC3339Nano)
		prompt := strings.Replace(helpers.UserPrompt(sid, syntheticCWD, ts), `"content":"hi"`, `"content":"`+output+`"`, 1)
		reply := helpers.AssistantReply(sid, fmt.Sprintf("msg_%s_%d", sid[:8], turns), ts)
		n, err := fmt.Fprintf(w, "%s\n%s\n", prompt, reply)
		require.NoError(t, err)
		written += int64(n)
	}
	require.NoError(t, w.Flush())
	require.NoError(t, f.Close())
	return written, turns
}

// bytesRead is what the process has read, from any file, in bytes.
func bytesRead(t *testing.T, pid int) int64 {
	t.Helper()
	io, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "io"))
	require.NoError(t, err)
	for line := range strings.SplitSeq(string(io), "\n") {
		if v, ok := strings.CutPrefix(line, "rchar: "); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			require.NoError(t, err)
			return n
		}
	}
	require.FailNow(t, "no rchar in /proc/pid/io")
	return 0
}
