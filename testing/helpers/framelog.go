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
}

// Looks is what the city's look cache did in a frame: looks blitted,
// drawn straight, and baked, and the bytes it holds (item 192).
type Looks struct {
	Hits, Misses, Bakes, Bytes int
}

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
	}
	return worst
}
