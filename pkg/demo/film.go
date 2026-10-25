package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"math"
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
	// View films the city in one of its views, by name, reached by the
	// lens keys a person would press (item 76).
	View    string `yaml:"view"`
	Signage string `yaml:"signage"`
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
	// PosterAt is the second of the clip its poster is taken from; unset,
	// a third of the way in (item 76).
	PosterAt *float64 `yaml:"poster_at"`
	// Loop blends the clip's end into its start; MaxMB is the most any
	// of its files may weigh, re-encoded smaller until it fits; Crops
	// are extra cuts from the middle of the frame, such as 4:5 for a
	// phone (item 76).
	Loop  *Loop    `yaml:"loop"`
	MaxMB float64  `yaml:"max_mb"`
	Crops []string `yaml:"crops"`
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

// withAnchors asks the city to write, beside a still, where each named
// thing landed in it (item 76).
func withAnchors(path string) cityOption {
	return func(args *[]string) { *args = append(*args, "--anchors", path) }
}

// anchorsFile is where a still's anchors are written, beside it.
func anchorsFile(still string) string {
	return strings.TrimSuffix(still, filepath.Ext(still)) + ".anchors.json"
}

// Anchor is where a named thing landed in a still, in its pixels.
type Anchor struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// readAnchors takes what the city wrote beside a still into the
// manifest, and removes the file: the manifest is where they live.
func readAnchors(path string) (map[string]Anchor, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var anchors map[string]Anchor
	if err := json.Unmarshal(data, &anchors); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return anchors, os.Remove(path)
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
	keys, _ := viewKeys(shot.View)
	if keys = append(keys, shot.Keys...); len(keys) > 0 {
		args = append(args, "--keys", strings.Join(keys, ","))
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
			"--seconds", strconv.FormatFloat(recordedSeconds(shot), 'g', -1, 64),
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
func encodeArgs(format, frames string, fps int, out string, opts ...encodeOption) ([]string, error) {
	var e encoding
	for _, o := range opts {
		o(&e)
	}
	args := []string{"-loglevel", "error", "-y", "-framerate", strconv.Itoa(fps), "-i", filepath.Join(frames, "frame-%05d.png")}
	if e.crop != "" {
		w, h, err := ratio(e.crop)
		if err != nil {
			return nil, err
		}
		args = append(args, "-vf", fmt.Sprintf("crop=trunc(ih*%d/%d/2)*2:ih", w, h))
	}
	crf := func() string { return strconv.Itoa(firstCRF(e.crf, defaultCRF[format])) }
	switch format {
	case "mp4":
		args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", crf(), "-movflags", "+faststart")
	case "webm":
		args = append(args, "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", "-crf", crf(), "-b:v", "0")
	case "gif":
		args = append(args, "-vf", "fps=15,scale=960:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=160[p];[s1][p]paletteuse=dither=bayer:bayer_scale=4")
	default:
		return nil, fmt.Errorf("format %q: want mp4, webm or gif", format)
	}
	return append(args, out), nil
}

// defaultCRF is each video format's quality when a shot has no budget
// to keep to: lower is better and bigger.
var defaultCRF = map[string]int{"mp4": 20, "webm": 34}

// maxCRFRise is how far above its default a budgeted file's CRF may be
// raised before the film gives up on fitting it, in steps of crfStep.
const (
	maxCRFRise = 16
	crfStep    = 2
)

type encoding struct {
	crf  int
	crop string
}

type encodeOption func(*encoding)

func withCRF(crf int) encodeOption      { return func(e *encoding) { e.crf = crf } }
func withCrop(crop string) encodeOption { return func(e *encoding) { e.crop = crop } }

func firstCRF(crf, fallback int) int {
	if crf > 0 {
		return crf
	}
	return fallback
}

// encodeWithin encodes a file at its default CRF and, while it weighs
// more than the budget, again at a higher one, up to maxCRFRise above.
// encode makes the file at a CRF and says how big it came out.
func encodeWithin(file string, crf int, budget int64, encode func(crf int) (int64, error)) error {
	var size int64
	for rise := 0; rise <= maxCRFRise; rise += crfStep {
		var err error
		if size, err = encode(crf + rise); err != nil {
			return err
		}
		if budget <= 0 || size <= budget {
			return nil
		}
	}
	return fmt.Errorf("%s: still %.1f MB at CRF %d, %d above its usual %d; the budget is %.1f MB",
		file, float64(size)/(1<<20), crf+maxCRFRise, maxCRFRise, crf, float64(budget)/(1<<20))
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
	View        string  `json:"view,omitempty"`
	// Crop is the shape cut from the middle of the frame, such as "4:5".
	Crop string `json:"crop,omitempty"`
	// Anchors is where each named thing landed in a still: the plant,
	// "tower:<server>", "district:<name>" and the rest.
	Anchors map[string]Anchor `json:"anchors,omitempty"`
}

// shotMedia is what a shot's manifest entries share, whatever files it
// makes.
func shotMedia(scenario Scenario, shot Shot) Media {
	return Media{Scenario: scenario.Name, Shot: shot.Name, Title: shot.Title, Description: shot.Description, View: shot.View}
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
			media, ok := recall(f.Out, scenario.Name, shot, key)
			if !ok && f.Adopt {
				if media, ok = adopt(f.Out, scenario.Name, shot, key); !ok {
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

// filmHold is how long a shot's stand-ins live if the filmer dies before
// unstaging them. A clip's timeline runs in video time, so on a loaded
// machine one clip can film for longer than an hour; a hold shorter than
// the film turns its sessions resting mid-clip (item 187).
const filmHold = 24 * time.Hour

// shotUnits is a shot's share of the run's progress: its frames, or a
// still's worth of them.
func shotUnits(shot Shot) int {
	if shot.Record == nil {
		return stillUnits
	}
	return int(recordedSeconds(shot) * float64(fpsOf(shot.Record)))
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
	director, err := NewDirector(f.Corpus, scenario, home, f.Now(), Sleepers(home, filmHold))
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
	media := shotMedia(scenario, shot)
	fmt.Fprintf(f.Log, "filming %s/%s\n", scenario.Name, shot.Name)
	progress.Detail("filming")
	if shot.Record == nil {
		still := filepath.Join(dir, shot.Name+".png")
		if err := f.city(work, tz, cityArgs(shot, home, still, "", withAnchors(anchorsFile(still)))); err != nil {
			return nil, err
		}
		if media.Anchors, err = readAnchors(anchorsFile(still)); err != nil {
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
	fps := fpsOf(shot.Record)
	if shot.Loop != nil {
		progress.Detail("looping")
		looped := filepath.Join(work, "looped")
		if err := os.MkdirAll(looped, 0o700); err != nil {
			return nil, err
		}
		seconds := func(s float64) int { return int(math.Round(s * float64(fps))) }
		if err := loopFrames(frames, looped, seconds(LoopLead), seconds(shot.Record.Seconds), seconds(shot.Loop.Crossfade)); err != nil {
			return nil, err
		}
		frames = looped
	}
	poster := filepath.Join(dir, shot.Name+".png")
	if err := posterFrom(frames, poster, fps, shot.PosterAt); err != nil {
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
	for _, crop := range append([]string{""}, shot.Crops...) {
		name, still, w := shot.Name, poster, width
		if crop != "" {
			cw, ch, err := ratio(crop)
			if err != nil {
				return nil, err
			}
			name += "-" + cropName(crop)
			still = filepath.Join(dir, name+".png")
			if err := cropPoster(poster, still, crop); err != nil {
				return nil, err
			}
			w = cropWidth(height, cw, ch)
		}
		for _, format := range formats {
			file := filepath.Join(dir, name+"."+format)
			progress.Detail(strings.TrimSpace("encoding " + cropName(crop) + " " + format))
			err := encodeWithin(file, defaultCRF[format], int64(shot.MaxMB*(1<<20)), func(crf int) (int64, error) {
				args, err := encodeArgs(format, frames, fps, file, withCRF(crf), withCrop(crop))
				if err != nil {
					return 0, err
				}
				if err := run(exec.Command(f.FFmpeg, args...), f.Log); err != nil {
					return 0, fmt.Errorf("ffmpeg %s: %w", format, err)
				}
				info, err := os.Stat(file)
				if err != nil {
					return 0, err
				}
				return info.Size(), nil
			})
			if err != nil {
				return nil, err
			}
			m := media
			m.File, m.Kind, m.Format, m.Poster, m.Crop = rel(f.Out, file), "video", format, rel(f.Out, still), crop
			m.Width, m.Height, m.Seconds = w, height, shot.Record.Seconds
			if format == "gif" {
				m.Kind = "animation"
				m.Width, m.Height = 960, height*960/width
			}
			out = append(out, m)
		}
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

// posterFrom keeps one frame as the clip's still: the frame at the
// shot's poster_at, or a third of the way in, since the first frame is
// the city before its fit has settled.
func posterFrom(frames, poster string, fps int, at *float64) error {
	all, err := filepath.Glob(filepath.Join(frames, "frame-*.png"))
	if err != nil || len(all) == 0 {
		return fmt.Errorf("no frames recorded in %s", frames)
	}
	data, err := os.ReadFile(all[posterFrame(len(all), fps, at)])
	if err != nil {
		return err
	}
	return os.WriteFile(poster, data, 0o644)
}

// posterFrame is the index of the poster among a clip's frames.
func posterFrame(frames, fps int, at *float64) int {
	if at == nil {
		return frames / 3
	}
	return max(0, min(frames-1, int(math.Round(*at*float64(fps)))))
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
