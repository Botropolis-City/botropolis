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
	const (
		maxCores   = 1.0
		dirtyLimit = 256 << 20
	)
	history := int64(256 << 20)
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		history = int64(gb * (1 << 30))
	}

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over %d MiB of history", history>>20), func(t *testing.T) {
		l := newLaunch(t, history, 8<<20)
		run := l.start(t)
		run.untilIdle(t)
		read := bytesRead(t, run.pid)
		t.Logf("%d MiB of history in %s: peak %.0f%% of a core, peak %d MiB Private_Dirty, %d MiB read",
			l.written>>20, run.took.Round(time.Second), run.peakShare*100, run.peakDirty>>20, read>>20)

		t.Run("it should read the whole history", func(t *testing.T) {
			assert.GreaterOrEqual(t, read, l.written)
		})

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, run.peakShare, maxCores)
		})

		t.Run("it should stay under 256 MiB of Private_Dirty at every sample", func(t *testing.T) {
			assert.Less(t, run.peakDirty, int64(dirtyLimit))
		})
	})
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
		window        = 2 * 64 << 10
		slack         = 16 << 20
		maxCPUSeconds = 2.0
		clockTicksSec = 100
		maxCores      = 1.0
		stopped       = 400
	)
	scale := 1.0
	if gb, err := strconv.ParseFloat(os.Getenv(firstLaunchGB), 64); err == nil && gb > 0 {
		scale = gb * 1024 / 484
	}
	sized := func(n int64) int64 { return int64(float64(n) * scale) }
	old := sized(256 << 10)
	liveSizes := []int64{sized(288 << 20), sized(16 << 20)}
	todaySizes := []int64{sized(64 << 20), sized(16 << 20)}
	now := time.Now()

	t.Run(fmt.Sprintf("when the daemon starts on an empty state directory over a home shaped like a real one: %d stopped sessions of %d KiB, live sessions of %d and %d MiB, and stopped ones active today of %d and %d MiB", stopped, old>>10, liveSizes[0]>>20, liveSizes[1]>>20, todaySizes[0]>>20, todaySizes[1]>>20), func(t *testing.T) {
		root := t.TempDir()
		home := helpers.HomeAt(t, filepath.Join(root, "home"))
		at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for s := range stopped {
			syntheticSession(t, home, syntheticSID(s), at.Add(time.Duration(s)*time.Hour), old)
		}
		var live []string
		turns := 0
		for i, pid := range []int{os.Getpid(), os.Getppid()} {
			sid := syntheticSID(stopped + i)
			home.Session(pid, sid, syntheticCWD, "interactive", "idle")
			_, n := syntheticSession(t, home, sid, now.Add(-3*time.Hour), liveSizes[i])
			live, turns = append(live, sid), turns+n
		}
		for i, size := range todaySizes {
			_, n := syntheticSession(t, home, syntheticSID(stopped+2+i), now.Add(-20*time.Hour), size)
			turns += n
		}
		l := &launch{root: root, home: home.Path}
		run := l.start(t)
		idle := make(chan struct{})
		go func() {
			defer close(idle)
			run.untilIdle(t)
		}()
		var client *proto.Client
		require.Eventually(t, func() bool {
			c, err := proto.Dial(run.socket)
			client = c
			return err == nil
		}, time.Minute, 10*time.Millisecond)
		_, err := client.Snapshot()
		require.NoError(t, err)
		readFirst := bytesRead(t, run.pid)
		cpuFirst := float64(cpuTicks(t, run.pid)) / clockTicksSec
		_ = client.Close()
		<-idle
		client, err = proto.Dial(run.socket)
		require.NoError(t, err)
		settled, err := client.Snapshot()
		require.NoError(t, err)
		_ = client.Close()
		transcripts := int64(stopped + 4)
		t.Logf("first snapshot after %d MiB and %.2f CPU-seconds; idle after %s at a peak of %.0f%% of a core",
			readFirst>>20, cpuFirst, run.took.Round(time.Second), run.peakShare*100)

		t.Run("it should read no more than each transcript's window before its first snapshot", func(t *testing.T) {
			assert.LessOrEqual(t, readFirst, transcripts*window+slack)
		})

		if scale != 1 {
			t.Run("it should send its first snapshot within 2 CPU-seconds", func(t *testing.T) {
				assert.LessOrEqual(t, cpuFirst, maxCPUSeconds)
			})
		}

		t.Run("it should use at most one core in any 1 s sample", func(t *testing.T) {
			assert.LessOrEqual(t, run.peakShare, maxCores)
		})

		t.Run("and it has settled", func(t *testing.T) {
			for _, sid := range live {
				t.Run("it should hold live session "+sid[:8]+"'s usage whole", func(t *testing.T) {
					assert.False(t, sessionIn(t, settled, sid).Usage.Partial)
				})
			}

			t.Run("it should give the plant the day's whole output", func(t *testing.T) {
				assert.Equal(t, int64(5*turns), settled.Power.Output)
			})
		})
	})
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
		uninterrupted.untilIdle(t)
		once := bytesRead(t, uninterrupted.pid)
		again := once - whole.written

		l := newLaunch(t, history, perSession)
		first := l.start(t)
		require.Eventually(t, func() bool { return bytesRead(t, first.pid) >= stopAt }, 5*time.Minute, 50*time.Millisecond)
		first.kill(t)
		before := first.read
		second := l.start(t)
		second.untilIdle(t)
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
	const (
		within        = 5 * time.Second
		maxCPUSeconds = 1.0
		clockTicksSec = 100
	)
	timed := os.Getenv("BOTROPOLIS_FIRST_LAUNCH_TIMED") != ""

	t.Run("when the city opens on a daemon partway through a first launch", func(t *testing.T) {
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
		var log strings.Builder
		var firstDrawn, firstReading time.Duration
		cpuToDrawn := -1.0
		lines := bufio.NewScanner(out)
		lines.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for lines.Scan() {
			line := lines.Text()
			log.WriteString(line + "\n")
			if strings.HasPrefix(line, "still: ") && cpuToDrawn < 0 {
				firstDrawn = time.Since(start)
				cpuToDrawn = float64(cpuTicks(t, city.Process.Pid)) / clockTicksSec
			}
			if strings.HasPrefix(line, "reading: ") && firstReading == 0 {
				firstReading = time.Since(start)
			}
		}
		_ = city.Wait()
		var shown []string
		for _, f := range helpers.ParseFrameLog(strings.NewReader(log.String())) {
			if f.Reading != "" || len(shown) > 0 {
				shown = append(shown, f.Reading)
			}
		}
		for len(shown) > 0 && shown[len(shown)-1] == "" {
			shown = shown[:len(shown)-1]
		}
		var percents []int
		for _, line := range shown {
			if m := percentIn.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				percents = append(percents, n)
			}
		}
		t.Logf("first city drawn at %s after %.2f CPU-seconds; first reading line at %s; %d frames from it", firstDrawn.Round(time.Millisecond), cpuToDrawn, firstReading.Round(time.Millisecond), len(shown))
		if len(shown) == 0 {
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

		t.Run("it should show a percentage", func(t *testing.T) {
			require.NotEmpty(t, shown)
			assert.Contains(t, shown[0], "%")
		})

		t.Run("it should show it on every frame until the read is done", func(t *testing.T) {
			assert.NotContains(t, shown, "")
		})

		t.Run("it should move as the read goes on", func(t *testing.T) {
			require.NotEmpty(t, percents)
			assert.Less(t, percents[0], percents[len(percents)-1])
		})

		t.Run("it should never go backwards", func(t *testing.T) {
			assert.True(t, slices.IsSorted(percents))
		})

		t.Run("it should spend at most 1 CPU-second before it draws the city", func(t *testing.T) {
			assert.True(t, cpuToDrawn >= 0 && cpuToDrawn <= maxCPUSeconds)
		})

		if timed {
			t.Run("it should draw the city within 5 s", func(t *testing.T) {
				assert.True(t, firstDrawn > 0 && firstDrawn <= within)
			})

			t.Run("it should show the percentage within 5 s", func(t *testing.T) {
				assert.True(t, firstReading > 0 && firstReading <= within)
			})
		}
	})
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

// untilIdle samples the daemon each second until it has used under 5%
// of a core for three samples running.
func (r *daemonRun) untilIdle(t *testing.T) {
	t.Helper()
	const (
		clockTicksSec = 100
		idleShare     = 0.05
		idleSamples   = 3
		gaveUp        = 10 * time.Minute
	)
	quiet := 0
	start := time.Now()
	last, at := cpuTicks(t, r.pid), time.Now()
	for quiet < idleSamples && time.Since(start) < gaveUp {
		time.Sleep(time.Second)
		ticks, now := cpuTicks(t, r.pid), time.Now()
		share := float64(ticks-last) / clockTicksSec / now.Sub(at).Seconds()
		last, at = ticks, now
		r.peakShare = max(r.peakShare, share)
		r.peakDirty = max(r.peakDirty, privateDirty(r.pid))
		if share < idleShare {
			quiet++
		} else {
			quiet = 0
		}
	}
	r.took = time.Since(start)
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
