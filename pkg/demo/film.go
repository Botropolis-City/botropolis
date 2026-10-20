package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"botropolis.city/botropolis/pkg/script"
)

// Shot is one piece of media made from a scenario: a still, or a clip
// when Record is set.
type Shot struct {
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Window      string   `yaml:"window"`
	Keys        []string `yaml:"keys"`
	Hover       string   `yaml:"hover"`
	Detail      string   `yaml:"detail"`
	Signage     string   `yaml:"signage"`
	// DistrictType films every new district as this type, or by the
	// district_type setting's other modes (item 109).
	DistrictType string `yaml:"district_type"`
	// GroupLines shows or hides the group lines for this shot; unset, the
	// city's default stands.
	GroupLines *bool `yaml:"group_lines"`
	// Clock overrides the scenario's time of day for this shot.
	Clock string `yaml:"clock"`
	// Scale is the render scale; 0 follows the virtual display, which
	// is 1.
	Scale   float64    `yaml:"scale"`
	Record  *Recording `yaml:"record"`
	Formats []string   `yaml:"formats"`
	// Timeline plays the scenario's arrivals, departures and changes
	// while a clip records.
	Timeline bool `yaml:"timeline"`
	// Script is the clip's choreography: the pointer gliding to things
	// and clicking them, the wheel zooming, arrows panning.
	Script []script.Cue `yaml:"script"`
}

// Recording is how long a clip runs and at what frame rate.
type Recording struct {
	Seconds float64 `yaml:"seconds"`
	FPS     int     `yaml:"fps"`
}

const (
	defaultClock  = "14:00"
	defaultWindow = "1920x1080"
	defaultFPS    = 30
	// noDaemon is a socket that cannot exist, which sends the city
	// straight to the staged files instead of whatever daemon the
	// machine is running against the real home.
	noDaemon = "/nonexistent/botropolis.sock"
)

var defaultFormats = []string{"mp4", "webm", "gif"}

type cityOption func(*[]string)

func withSocket(sock string) cityOption {
	return func(args *[]string) { (*args)[5] = sock }
}

// cityArgs is the botropolis command line for one shot: a screenshot to
// still, or frames into frames.
func cityArgs(shot Shot, home, still, frames string, opts ...cityOption) []string {
	args := []string{"city", "--headless", "--home", home, "--socket", noDaemon}
	for _, o := range opts {
		o(&args)
	}
	window := shot.Window
	if window == "" {
		window = defaultWindow
	}
	args = append(args, "--window", window)
	if len(shot.Keys) > 0 {
		args = append(args, "--keys", strings.Join(shot.Keys, ","))
	}
	for _, f := range []struct{ flag, value string }{
		{"--hover", shot.Hover}, {"--detail", shot.Detail}, {"--signage", shot.Signage}, {"--district_type", shot.DistrictType},
	} {
		if f.value != "" {
			args = append(args, f.flag, f.value)
		}
	}
	if shot.GroupLines != nil {
		args = append(args, "--group_lines="+strconv.FormatBool(*shot.GroupLines))
	}
	if shot.Scale > 0 {
		args = append(args, "--render_scale", strconv.FormatFloat(shot.Scale, 'g', -1, 64))
	}
	if shot.Record != nil {
		if len(shot.Script) > 0 {
			args = append(args, "--script", filepath.Join(filepath.Dir(frames), "script.json"))
		}
		return append(args, "--record", frames,
			"--seconds", strconv.FormatFloat(shot.Record.Seconds, 'g', -1, 64),
			"--fps", strconv.Itoa(fpsOf(shot.Record)))
	}
	return append(args, "--screenshot", still)
}

func fpsOf(r *Recording) int {
	if r.FPS > 0 {
		return r.FPS
	}
	return defaultFPS
}

// encodeArgs is the ffmpeg command line that turns recorded frames into
// one web format. H.264 in MP4 and VP9 in WebM between them play in
// every current browser; the GIF is for places that take no video,
// such as a README.
func encodeArgs(format, frames string, fps int, out string) ([]string, error) {
	args := []string{"-loglevel", "error", "-y", "-framerate", strconv.Itoa(fps), "-i", filepath.Join(frames, "frame-%05d.png")}
	switch format {
	case "mp4":
		args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20", "-movflags", "+faststart")
	case "webm":
		args = append(args, "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", "-crf", "34", "-b:v", "0")
	case "gif":
		args = append(args, "-vf", "fps=15,scale=960:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=160[p];[s1][p]paletteuse=dither=bayer:bayer_scale=4")
	default:
		return nil, fmt.Errorf("format %q: want mp4, webm or gif", format)
	}
	return append(args, out), nil
}

// clockZone names a time zone in which now falls in the hour the clock
// asks for, so the city lights the scene for that time of day without
// anything in it being told to. Go reads TZ as a zone name only, so the
// zone is one of the Etc/GMT ones: Etc/GMT+N is N hours behind UTC, and
// they run from 12 behind to 14 ahead, which reaches every hour.
func clockZone(now time.Time, clock string) (string, error) {
	at, err := time.Parse("15:04", clock)
	if err != nil {
		return "", fmt.Errorf("clock %q: want HH:MM", clock)
	}
	ahead := ((at.Hour()-now.UTC().Hour())%24 + 24) % 24
	if ahead > 14 {
		ahead -= 24
	}
	switch {
	case ahead == 0:
		return "Etc/GMT", nil
	case ahead > 0:
		return fmt.Sprintf("Etc/GMT-%d", ahead), nil
	}
	return fmt.Sprintf("Etc/GMT+%d", -ahead), nil
}

// Manifest describes every file a film run made, for whatever builds the
// site to pick from without opening each one.
type Manifest struct {
	Generated time.Time `json:"generated"`
	Version   string    `json:"version"`
	Media     []Media   `json:"media"`
}

type Media struct {
	File        string  `json:"file"`
	Kind        string  `json:"kind"`
	Format      string  `json:"format"`
	Scenario    string  `json:"scenario"`
	Shot        string  `json:"shot"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Seconds     float64 `json:"seconds,omitempty"`
	Poster      string  `json:"poster,omitempty"`
}

// Filmer stages each scenario afresh for every shot, so each one sees
// its sessions exactly as long ago as the scenario says, and runs the
// city against it with every home-shaped variable pointed into a
// scratch directory.
type Filmer struct {
	Corpus     Corpus
	Botropolis string
	FFmpeg     string
	Out        string
	Version    string
	Log        io.Writer
	// Progress is told how far the run has got; nil reports nothing.
	Progress *Progress
	// Self is this binary, whose staging every shot depends on as much as
	// on the city's; Fresh films every shot whether or not it changed.
	Self  string
	Fresh bool
	// Source names the code by a hash of its source, so a cache filmed on
	// one machine is good on another; empty keys on the binaries instead.
	Source string
	// Adopt files every cached shot under its current key without
	// filming it; see adopt.
	Adopt bool
	Now   func() time.Time
}

// Film makes every shot of every scenario and writes the manifest.
func (f Filmer) Film(paths []string) (Manifest, error) {
	manifest := Manifest{Generated: f.Now().UTC(), Version: f.Version}
	paths, err := scenarioFiles(paths)
	if err != nil {
		return manifest, err
	}
	var scenarios []Scenario
	total := 0
	for _, path := range paths {
		scenario, err := LoadScenario(path)
		if err != nil {
			return manifest, err
		}
		if scenario.Name == "" {
			scenario.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		for _, shot := range scenario.Shots {
			total += shotUnits(shot)
		}
		scenarios = append(scenarios, scenario)
	}
	progress := f.Progress
	if progress == nil {
		progress = &Progress{now: time.Now}
	}
	progress.Total = total
	binaries := []string{f.Botropolis}
	if f.Self != "" {
		binaries = append(binaries, f.Self)
	}
	inputs, err := inputsKey(f.Corpus.Dir, f.Source, binaries...)
	if err != nil {
		return manifest, err
	}
	shots, n, reused := 0, 0, 0
	for _, s := range scenarios {
		shots += len(s.Shots)
	}
	for _, scenario := range scenarios {
		for _, shot := range scenario.Shots {
			n++
			name := fmt.Sprintf("%d/%d %s/%s", n, shots, scenario.Name, shot.Name)
			key := shotKey(scenario, shot, inputs)
			media, ok := recall(f.Out, scenario.Name, shot.Name, key)
			if !ok && f.Adopt {
				if media, ok = adopt(f.Out, scenario.Name, shot.Name, key); !ok {
					return manifest, fmt.Errorf("%s/%s: nothing cached to adopt; film it without --adopt", scenario.Name, shot.Name)
				}
			}
			if ok && !f.Fresh {
				progress.Step(name, "unchanged")
				progress.Skip(shotUnits(shot))
				manifest.Media = append(manifest.Media, media...)
				reused++
				continue
			}
			progress.Step(name, "staging")
			media, err := f.shoot(scenario, shot, progress)
			if err != nil {
				return manifest, fmt.Errorf("%s/%s: %w", scenario.Name, shot.Name, err)
			}
			if err := remember(f.Out, scenario.Name, shot.Name, key, media); err != nil {
				return manifest, err
			}
			manifest.Media = append(manifest.Media, media...)
		}
	}
	if err := prune(f.Out, manifest); err != nil {
		return manifest, err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	progress.Finish(fmt.Sprintf("filmed %d of %d shots, %d unchanged; %d files", shots-reused, shots, reused, len(manifest.Media)))
	return manifest, writePublic(filepath.Join(f.Out, "manifest.json"), append(data, '\n'))
}

// stillUnits is what a still costs against a clip's frames, measured:
// a still takes about as long as six recorded frames, staging included.
const stillUnits = 6

// shotUnits is a shot's share of the run's progress: its frames, or a
// still's worth of them.
func shotUnits(shot Shot) int {
	if shot.Record == nil {
		return stillUnits
	}
	return int(shot.Record.Seconds * float64(fpsOf(shot.Record)))
}

// scenarioFiles expands each directory to the scenario files in it, in
// name order, so a run films the same things in the same order.
func scenarioFiles(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, p)
			continue
		}
		found, err := filepath.Glob(filepath.Join(p, "*.yaml"))
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

func (f Filmer) shoot(scenario Scenario, shot Shot, progress *Progress) ([]Media, error) {
	work, err := os.MkdirTemp("", "botropolis-film-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	home := filepath.Join(work, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	director, err := NewDirector(f.Corpus, scenario, home, f.Now(), Sleepers(home, time.Hour))
	if err != nil {
		return nil, err
	}
	defer func() { _ = Unstage(home) }()
	dir := filepath.Join(f.Out, scenario.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	clock := firstOf(shot.Clock, scenario.Clock, defaultClock)
	tz, err := clockZone(f.Now(), clock)
	if err != nil {
		return nil, err
	}
	media := Media{Scenario: scenario.Name, Shot: shot.Name, Title: shot.Title, Description: shot.Description}
	fmt.Fprintf(f.Log, "filming %s/%s\n", scenario.Name, shot.Name)
	progress.Detail("filming")
	if shot.Record == nil {
		still := filepath.Join(dir, shot.Name+".png")
		if err := f.city(work, tz, cityArgs(shot, home, still, "")); err != nil {
			return nil, err
		}
		media.File, media.Kind, media.Format = rel(f.Out, still), "image", "png"
		media.Width, media.Height, err = size(still)
		progress.Advance(stillUnits)
		return []Media{media}, err
	}
	frames := filepath.Join(work, "frames")
	if err := os.MkdirAll(frames, 0o700); err != nil {
		return nil, err
	}
	if len(shot.Script) > 0 {
		if err := script.Save(filepath.Join(work, "script.json"), shot.Script); err != nil {
			return nil, err
		}
	}
	stop := watchFrames(frames, shotUnits(shot), progress)
	err = f.record(work, tz, cityArgs(shot, home, "", frames), frames, fpsOf(shot.Record), director, shot.Timeline)
	stop()
	if err != nil {
		return nil, err
	}
	poster := filepath.Join(dir, shot.Name+".png")
	if err := posterFrom(frames, poster); err != nil {
		return nil, err
	}
	width, height, err := size(poster)
	if err != nil {
		return nil, err
	}
	formats := shot.Formats
	if len(formats) == 0 {
		formats = defaultFormats
	}
	var out []Media
	for _, format := range formats {
		file := filepath.Join(dir, shot.Name+"."+format)
		args, err := encodeArgs(format, frames, fpsOf(shot.Record), file)
		if err != nil {
			return nil, err
		}
		progress.Detail("encoding " + format)
		if err := run(exec.Command(f.FFmpeg, args...), f.Log); err != nil {
			return nil, fmt.Errorf("ffmpeg %s: %w", format, err)
		}
		m := media
		m.File, m.Kind, m.Format, m.Poster = rel(f.Out, file), "video", format, rel(f.Out, poster)
		m.Width, m.Height, m.Seconds = width, height, shot.Record.Seconds
		if format == "gif" {
			m.Kind = "animation"
			m.Width, m.Height = 960, height*960/width
		}
		out = append(out, m)
	}
	return out, nil
}

// city runs botropolis with HOME, the XDG directories and the config
// all inside the work dir: the city's layout file, its settings and
// anything else it reaches for by home directory land there, never in
// the real ones.
func (f Filmer) city(work, tz string, args []string) error {
	return run(f.cityCommand(work, tz, args), f.Log)
}

// record runs a clip, playing the timeline against the staged home
// while the city records when asked to. The timeline follows the
// frames, not the wall clock: an event at 6s happens once the frame six
// seconds into the clip has been written. The recorder writes a frame
// every few ticks, and on software GL a tick takes longer than it
// should, so wall-clock timing ran the whole timeline before the city
// had recorded a second of it. Found on film, twice.
func (f Filmer) record(work, tz string, args []string, frames string, fps int, director *Director, timeline bool) error {
	cmd := f.cityCommand(work, tz, args)
	cmd.Stdout, cmd.Stderr = f.Log, f.Log
	if err := cmd.Start(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	played := make(chan error, 1)
	go func() {
		if !timeline {
			played <- nil
			return
		}
		played <- director.PlayFrames(ctx, frames, fps)
	}()
	err := cmd.Wait()
	cancel()
	if playErr := <-played; err == nil {
		err = playErr
	}
	return err
}

func (f Filmer) cityCommand(work, tz string, args []string) *exec.Cmd {
	cmd := exec.Command(f.Botropolis, args...)
	env := []string{
		"HOME=" + filepath.Join(work, "home"),
		"XDG_STATE_HOME=" + filepath.Join(work, "state"),
		"XDG_CONFIG_HOME=" + filepath.Join(work, "config"),
		"XDG_CACHE_HOME=" + filepath.Join(work, "cache"),
		"XDG_RUNTIME_DIR=" + filepath.Join(work, "run"),
		"TZ=" + tz,
	}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "PATH", "LANG", "LC_ALL", "TMPDIR":
			env = append(env, kv)
		}
	}
	cmd.Env = env
	return cmd
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func run(cmd *exec.Cmd, log io.Writer) error {
	cmd.Stdout, cmd.Stderr = log, log
	return cmd.Run()
}

// watchFrames counts a clip's frames as the city writes them, for the
// progress bar, until the returned stop is called; then it settles the
// count at the clip's full share, so a short clip does not leave the
// total behind.
func watchFrames(frames string, expected int, progress *Progress) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	seen := 0
	go func() {
		defer close(finished)
		for {
			select {
			case <-done:
				return
			case <-time.After(500 * time.Millisecond):
			}
			all, _ := filepath.Glob(filepath.Join(frames, "frame-*.png"))
			if n := min(len(all), expected); n > seen {
				progress.Detail(fmt.Sprintf("frame %d/%d", n, expected))
				progress.Advance(n - seen)
				seen = n
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		if seen < expected {
			progress.Advance(expected - seen)
		}
	}
}

// posterFrom keeps the frame a third of the way in as the clip's
// still: the first frame is the city before its fit has settled.
func posterFrom(frames, poster string) error {
	all, err := filepath.Glob(filepath.Join(frames, "frame-*.png"))
	if err != nil || len(all) == 0 {
		return fmt.Errorf("no frames recorded in %s", frames)
	}
	data, err := os.ReadFile(all[len(all)/3])
	if err != nil {
		return err
	}
	return os.WriteFile(poster, data, 0o644)
}

func size(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = file.Close() }()
	config, _, err := image.DecodeConfig(file)
	return config.Width, config.Height, err
}

func rel(base, path string) string {
	r, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}
