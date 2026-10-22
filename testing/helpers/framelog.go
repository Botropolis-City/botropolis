package helpers

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Frame is the work one frame asked of the GPU, read from Ebitengine's
// own command log (built with -tags ebitenginedebug). Item 194.
type Frame struct {
	Commands, Indices, Switches int
	Looks                       Looks
	Still                       Still
	// Reading is the footer's reading line, when the frame showed one
	// (item 159).
	Reading string
	// Drawn is whether the city drew the frame, which it says by logging
	// its standing city's line.
	Drawn bool
}

// CityFrames is frames from the first the city drew: those before it are
// the window putting its art up, and how many there are moves with the
// art (item 162).
func CityFrames(frames []Frame) []Frame {
	for i, f := range frames {
		if f.Drawn {
			return frames[i:]
		}
	}
	return nil
}

// Looks is what the city's look cache did in a frame: looks blitted,
// drawn straight, and baked, and the bytes it holds (item 192).
type Looks struct {
	Hits, Misses, Bakes, Bytes int
}

// Still is what the city's standing city cost in a frame: composed
// whole, composed again in rectangles, the group lines composed, the
// rectangles of movers drawn again, the ground composed, whether the
// city was drawn straight while its camera moved, and how many layers a
// whole-pixel pan shifted (item 193).
type Still struct {
	Composes, Redos, Overlays, Rects, Ground, Direct, Shifts int
}

var stillLine = regexp.MustCompile(`^still: composes=(\d+) redos=(\d+) overlays=(\d+) rects=(\d+) ground=(\d+)(?: direct=(\d+) shifts=(\d+))?`)

var looksLine = regexp.MustCompile(`^looks: hits=(\d+) misses=(\d+) bakes=(\d+) bytes=(\d+)`)

var drawTriangles = regexp.MustCompile(`^  draw-triangles: dst: (\d+).*, num of indices: (\d+),`)

// ParseFrameLog splits Ebitengine's debug log at each "----" into
// frames and counts each frame's draw commands, their indices, and how
// often the destination image changes.
func ParseFrameLog(r io.Reader) []Frame {
	var frames []Frame
	var dst string
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "----") {
			frames = append(frames, Frame{})
			dst = ""
			continue
		}
		if l := looksLine.FindStringSubmatch(line); l != nil && len(frames) > 0 {
			n := func(i int) int { v, _ := strconv.Atoi(l[i]); return v }
			frames[len(frames)-1].Looks = Looks{Hits: n(1), Misses: n(2), Bakes: n(3), Bytes: n(4)}
			continue
		}
		if r, ok := strings.CutPrefix(line, "reading: "); ok && len(frames) > 0 {
			frames[len(frames)-1].Reading = r
			continue
		}
		if l := stillLine.FindStringSubmatch(line); l != nil && len(frames) > 0 {
			n := func(i int) int { v, _ := strconv.Atoi(l[i]); return v }
			frames[len(frames)-1].Still = Still{Composes: n(1), Redos: n(2), Overlays: n(3), Rects: n(4), Ground: n(5), Direct: n(6), Shifts: n(7)}
			frames[len(frames)-1].Drawn = true
			continue
		}
		m := drawTriangles.FindStringSubmatch(line)
		if m == nil || len(frames) == 0 {
			continue
		}
		f := &frames[len(frames)-1]
		f.Commands++
		n, _ := strconv.Atoi(m[2])
		f.Indices += n
		if m[1] != dst {
			f.Switches++
			dst = m[1]
		}
	}
	return frames
}

// MostWork is each count's largest over frames.
func MostWork(frames []Frame) Frame {
	var worst Frame
	for _, f := range frames {
		worst.Commands = max(worst.Commands, f.Commands)
		worst.Indices = max(worst.Indices, f.Indices)
		worst.Switches = max(worst.Switches, f.Switches)
		worst.Looks.Hits = max(worst.Looks.Hits, f.Looks.Hits)
		worst.Looks.Misses = max(worst.Looks.Misses, f.Looks.Misses)
		worst.Looks.Bakes = max(worst.Looks.Bakes, f.Looks.Bakes)
		worst.Looks.Bytes = max(worst.Looks.Bytes, f.Looks.Bytes)
		worst.Still.Composes = max(worst.Still.Composes, f.Still.Composes)
		worst.Still.Redos = max(worst.Still.Redos, f.Still.Redos)
		worst.Still.Overlays = max(worst.Still.Overlays, f.Still.Overlays)
		worst.Still.Rects = max(worst.Still.Rects, f.Still.Rects)
		worst.Still.Ground = max(worst.Still.Ground, f.Still.Ground)
		worst.Still.Direct = max(worst.Still.Direct, f.Still.Direct)
		worst.Still.Shifts = max(worst.Still.Shifts, f.Still.Shifts)
	}
	return worst
}
