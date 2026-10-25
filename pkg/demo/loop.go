package demo

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Loop makes a clip whose last frame leads into its first (item 76): its
// last Crossfade seconds are blended toward the frames that come just
// before its first, so playing it again continues rather than jumps.
type Loop struct {
	Crossfade float64 `yaml:"crossfade"`
}

// LoopLead is the seconds a loop records before anything it keeps, for
// the city's fit to settle; a looping clip's script runs in recorded
// time, so its first kept frame is LoopLead plus the crossfade in.
const LoopLead = 1.0

// recordedSeconds is how long the city records for a shot.
func recordedSeconds(shot Shot) float64 {
	if shot.Loop == nil {
		return shot.Record.Seconds
	}
	return LoopLead + shot.Record.Seconds + shot.Loop.Crossfade
}

// loopFrames writes n frames to dst from a recording in src of lead
// frames, then n, then c. Out frame i is recorded frame lead+c+i; the
// last c are blended toward recorded frames lead to lead+c-1, each by
// (j+1)/(c+1), so the frame after the last, out frame 0, is recorded
// frame lead+c, which follows lead+c-1.
func loopFrames(src, dst string, lead, n, c int) error {
	frame := func(dir string, i int) string { return filepath.Join(dir, fmt.Sprintf("frame-%05d.png", i)) }
	if _, err := os.Stat(frame(src, lead+n+c-1)); err != nil {
		return fmt.Errorf("loop: the recording is short of the %d frames it needs: %w", lead+n+c, err)
	}
	for i := range n {
		at := frame(src, lead+c+i)
		j := i - (n - c)
		if j < 0 {
			data, err := os.ReadFile(at)
			if err != nil {
				return err
			}
			if err := os.WriteFile(frame(dst, i), data, 0o644); err != nil {
				return err
			}
			continue
		}
		a, err := readPNG(at)
		if err != nil {
			return err
		}
		b, err := readPNG(frame(src, lead+j))
		if err != nil {
			return err
		}
		if err := writePNG(frame(dst, i), blend(a, b, float64(j+1)/float64(c+1))); err != nil {
			return err
		}
	}
	return nil
}

// blend is a weighted toward b by w.
func blend(a, b image.Image, w float64) *image.NRGBA {
	out := image.NewNRGBA(a.Bounds())
	draw.Draw(out, out.Bounds(), a, a.Bounds().Min, draw.Src)
	over := image.NewNRGBA(a.Bounds())
	draw.Draw(over, over.Bounds(), b, b.Bounds().Min, draw.Src)
	for i := range out.Pix {
		out.Pix[i] = uint8(float64(out.Pix[i])*(1-w) + float64(over.Pix[i])*w + 0.5)
	}
	return out
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return png.Decode(f)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ratio reads a crop such as "4:5".
func ratio(crop string) (w, h int, err error) {
	a, b, ok := strings.Cut(crop, ":")
	w, errW := strconv.Atoi(a)
	h, errH := strconv.Atoi(b)
	if !ok || errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("crop %q: want W:H, such as 4:5", crop)
	}
	return w, h, nil
}

// cropName is a crop's part of a file name: 4:5 is "4x5".
func cropName(crop string) string { return strings.ReplaceAll(crop, ":", "x") }

// cropWidth is the even width a crop takes from a frame height tall,
// the same as the encoder's trunc(ih*W/H/2)*2.
func cropWidth(height, w, h int) int { return height * w / h / 2 * 2 }

// cropPoster cuts the middle of a poster to a crop's shape, full height.
func cropPoster(src, dst, crop string) error {
	w, h, err := ratio(crop)
	if err != nil {
		return err
	}
	img, err := readPNG(src)
	if err != nil {
		return err
	}
	b := img.Bounds()
	width := cropWidth(b.Dy(), w, h)
	left := b.Min.X + (b.Dx()-width)/2
	out := image.NewNRGBA(image.Rect(0, 0, width, b.Dy()))
	draw.Draw(out, out.Bounds(), img, image.Pt(left, b.Min.Y), draw.Src)
	return writePNG(dst, out)
}
