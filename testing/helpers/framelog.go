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
}

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
	}
	return worst
}
