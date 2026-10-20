package render

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	imagedraw "image/draw"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
)

// A shared clip (item 168): an MP4 of a chosen length at thirty frames a
// second, the city's own rate, so every update is a frame. ffmpeg encodes
// it; without ffmpeg the clip says so and nothing starts.
const clipFPS = 30

// clipLengths are the lengths l steps through, in seconds; the second is
// where it starts.
var clipLengths = []int{5, 15, 30, 60}

// clipLength is the length the next clip will be.
func (s *shareMode) clipLength() int {
	return clipLengths[(1+s.clipStep)%len(clipLengths)]
}

// clipFormat is the encoder a clip is made with and the container it
// goes in.
type clipFormat struct{ codec, ext string }

// clipFormats are the encoders a clip can use, best first: H.264 in an
// MP4 plays anywhere, and VP9 in a WebM is what Fedora's own ffmpeg,
// ffmpeg-free, can make, since it leaves out libx264.
var clipFormats = []clipFormat{{codec: "libx264", ext: "mp4"}, {codec: "libvpx-vp9", ext: "webm"}}

// pickClipFormat chooses from what `ffmpeg -encoders` lists: one encoder
// a line, its flags and then its name.
func pickClipFormat(encoders string) (clipFormat, bool) {
	have := map[string]bool{}
	for _, line := range strings.Split(encoders, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 {
			have[fields[1]] = true
		}
	}
	for _, f := range clipFormats {
		if have[f.codec] {
			return f, true
		}
	}
	return clipFormat{}, false
}

// clipEncoders asks ffmpeg which encoders it has.
func clipEncoders(bin string) string {
	out, _ := exec.Command(bin, "-hide_banner", "-encoders").Output()
	return string(out)
}

// ffmpegPath finds ffmpeg.
func (g *Game) ffmpegPath() (string, error) {
	if g.findFFmpeg != nil {
		return g.findFFmpeg()
	}
	return exec.LookPath("ffmpeg")
}

// frameSink is where a recording's frames go: a GIF or a clip. Each says
// exactly once, on the channel finish or abandon returns, how it ended;
// an abandoned one leaves no file.
type frameSink interface {
	add(img image.Image)
	finish() <-chan error
	abandon() <-chan error
}

// clipWriter pipes frames to ffmpeg off the game loop. ffmpeg starts with
// the first frame, which says the size, and writes beside the clip until
// the last, so nothing watching the folder sees half an MP4.
type clipWriter struct {
	frames  chan image.Image
	done    chan error
	discard atomic.Bool
}

// clipQueue bounds the frames waiting for ffmpeg: at 1080p each is 8 MB.
const clipQueue = 16

func newClipWriter(bin, path string, fps int, format clipFormat) *clipWriter {
	w := &clipWriter{frames: make(chan image.Image, clipQueue), done: make(chan error, 1)}
	go func() { w.done <- w.run(bin, path, fps, format) }()
	return w
}

func (w *clipWriter) add(img image.Image) { w.frames <- img }

func (w *clipWriter) finish() <-chan error {
	close(w.frames)
	return w.done
}

func (w *clipWriter) abandon() <-chan error {
	w.discard.Store(true)
	close(w.frames)
	return w.done
}

func (w *clipWriter) run(bin, path string, fps int, format clipFormat) error {
	tmp := path + ".part"
	var (
		cmd     *exec.Cmd
		stdin   io.WriteCloser
		stderr  bytes.Buffer
		failed  error
		started bool
	)
	for img := range w.frames {
		if w.discard.Load() || failed != nil {
			continue
		}
		if cmd == nil {
			b := img.Bounds()
			cmd = exec.Command(bin, clipArgs(b.Dx(), b.Dy(), fps, format, tmp)...)
			cmd.Stderr = &stderr
			if stdin, failed = cmd.StdinPipe(); failed == nil {
				failed = cmd.Start()
			}
			started = failed == nil
			if !started {
				continue
			}
		}
		_, failed = stdin.Write(rgbaBytes(img))
	}
	if !started {
		if failed != nil {
			return fmt.Errorf("the clip could not start ffmpeg: %w", failed)
		}
		return nil
	}
	_ = stdin.Close()
	waited := cmd.Wait()
	if w.discard.Load() {
		_ = os.Remove(tmp)
		return nil
	}
	if failed == nil {
		failed = waited
	}
	if failed != nil {
		_ = os.Remove(tmp)
		return clipError(failed, stderr.String())
	}
	return os.Rename(tmp, path)
}

// clipArgs has ffmpeg read raw RGBA frames from its input and write a
// clip that plays anywhere: yuv420p and even dimensions, and for an MP4
// the index at the front, so a browser can start it before it has all of
// it.
func clipArgs(width, height, fps int, format clipFormat, out string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", width, height),
		"-r", fmt.Sprint(fps),
		"-i", "-",
		"-an",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-pix_fmt", "yuv420p",
	}
	if format.codec == "libvpx-vp9" {
		args = append(args, "-c:v", "libvpx-vp9", "-b:v", "0", "-crf", "32", "-deadline", "realtime", "-cpu-used", "8", "-row-mt", "1")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-movflags", "+faststart")
	}
	return append(args, "-f", format.ext, out)
}

// rgbaBytes is a frame's pixels, row after row, four bytes each.
func rgbaBytes(img image.Image) []byte {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) && rgba.Stride == 4*rgba.Rect.Dx() {
		return rgba.Pix
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	imagedraw.Draw(out, out.Bounds(), img, b.Min, imagedraw.Src)
	return out.Pix
}

// clipError says why ffmpeg failed, in its own last words when it left
// any.
func clipError(err error, stderr string) error {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if last := strings.TrimSpace(lines[len(lines)-1]); last != "" {
		return errors.New("the clip failed: ffmpeg said " + last)
	}
	return fmt.Errorf("the clip failed: %w", err)
}
