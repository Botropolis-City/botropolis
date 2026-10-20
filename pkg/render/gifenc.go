package render

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"slices"
	"sync/atomic"

	xdraw "golang.org/x/image/draw"
)

// A shared GIF (item 168): four seconds at fifteen frames a second, no
// wider than 720 pixels, which is the size a chat or an issue shows
// without scaling. It needs no encoder but the standard library's.
const (
	gifSeconds  = 4
	gifFPS      = 15
	gifMaxWidth = 720
)

// gifWriter shrinks, quantizes and collects a GIF's frames off the game
// loop, and writes the file once the last has come.
type gifWriter struct {
	frames chan image.Image
	done   chan error
	// discard is set when the recording is abandoned: nothing is written.
	discard atomic.Bool
}

// gifQueue bounds the frames waiting to be encoded, so a slow machine
// makes the loop wait rather than hold every frame's pixels.
const gifQueue = 8

func newGIFWriter(path string, fps, maxWidth int) *gifWriter {
	w := &gifWriter{frames: make(chan image.Image, gifQueue), done: make(chan error, 1)}
	go func() {
		anim := &gif.GIF{}
		for img := range w.frames {
			if w.discard.Load() {
				continue
			}
			i := len(anim.Image)
			anim.Image = append(anim.Image, quantize(shrinkTo(img, maxWidth)))
			anim.Delay = append(anim.Delay, frameDelay(i, fps))
		}
		if len(anim.Image) == 0 || w.discard.Load() {
			w.done <- nil
			return
		}
		w.done <- writeGIF(path, anim)
	}()
	return w
}

func (w *gifWriter) add(img image.Image) { w.frames <- img }

// finish says the last frame has come, and returns where the result
// will be told.
func (w *gifWriter) finish() <-chan error {
	close(w.frames)
	return w.done
}

// abandon ends the recording and writes nothing.
func (w *gifWriter) abandon() <-chan error {
	w.discard.Store(true)
	close(w.frames)
	return w.done
}

// frameDelay is frame i's delay in hundredths of a second, rounded so
// the delays add up to the clip's length exactly.
func frameDelay(i, fps int) int {
	at := func(n int) int { return (n*100 + fps/2) / fps }
	return at(i+1) - at(i)
}

func writeGIF(path string, anim *gif.GIF) error {
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gif.EncodeAll(f, anim); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// shrinkTo scales img down to maxWidth, keeping its shape; a narrower
// image is only copied.
func shrinkTo(img image.Image, maxWidth int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxWidth {
		w, h = maxWidth, h*maxWidth/w
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

// quantize fits a frame into one GIF palette by median cut: the colours
// are split, again and again, at the median of the box with the widest
// spread, until there are 256 boxes, and each box's colours become their
// average. A frame with 256 colours or fewer keeps them exactly; above
// that, colours are counted five bits a channel, which is fast, and each
// box still averages the true colours in it.
func quantize(img *image.RGBA) *image.Paletted {
	tallies := tally(img, pack)
	key := pack
	if len(tallies) > 256 {
		tallies, key = tally(img, pack5), pack5
	}
	colours := make([]uint32, 0, len(tallies))
	for c := range tallies {
		colours = append(colours, c)
	}
	slices.Sort(colours)
	var boxes [][]uint32
	if len(colours) <= 256 {
		for _, c := range colours {
			boxes = append(boxes, []uint32{c})
		}
	} else {
		boxes = medianCut(colours, tallies, 256)
	}
	palette := make(color.Palette, len(boxes))
	index := make(map[uint32]uint8, len(colours))
	for i, box := range boxes {
		palette[i] = average(box, tallies)
		for _, c := range box {
			index[c] = uint8(i)
		}
	}
	b := img.Bounds()
	out := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), palette)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := out.Pix[(y-b.Min.Y)*out.Stride:]
		for x := b.Min.X; x < b.Max.X; x++ {
			row[x-b.Min.X] = index[key(img.RGBAAt(x, y))]
		}
	}
	return out
}

// colourTally is how many pixels share a key, and their colours summed.
type colourTally struct{ n, r, g, b int }

func tally(img *image.RGBA, key func(color.RGBA) uint32) map[uint32]*colourTally {
	out := map[uint32]*colourTally{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			t := out[key(c)]
			if t == nil {
				t = &colourTally{}
				out[key(c)] = t
			}
			t.n++
			t.r += int(c.R)
			t.g += int(c.G)
			t.b += int(c.B)
		}
	}
	return out
}

func pack(c color.RGBA) uint32 { return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B) }

func pack5(c color.RGBA) uint32 { return pack(color.RGBA{c.R &^ 7, c.G &^ 7, c.B &^ 7, 0}) }

func channel(c uint32, ch int) uint8 { return uint8(c >> (16 - 8*ch)) }

func medianCut(colours []uint32, tallies map[uint32]*colourTally, want int) [][]uint32 {
	boxes := [][]uint32{colours}
	for len(boxes) < want {
		widest, ch, spread := -1, 0, 0
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			for c := range 3 {
				lo, hi := uint8(255), uint8(0)
				for _, col := range box {
					v := channel(col, c)
					lo, hi = min(lo, v), max(hi, v)
				}
				if int(hi-lo) > spread {
					widest, ch, spread = i, c, int(hi-lo)
				}
			}
		}
		if widest < 0 {
			break
		}
		box := boxes[widest]
		slices.SortFunc(box, func(a, b uint32) int { return int(channel(a, ch)) - int(channel(b, ch)) })
		total := 0
		for _, c := range box {
			total += tallies[c].n
		}
		cut, run := 1, 0
		for i, c := range box[:len(box)-1] {
			run += tallies[c].n
			if run*2 >= total {
				cut = i + 1
				break
			}
		}
		boxes = append(boxes[:widest], append([][]uint32{box[:cut], box[cut:]}, boxes[widest+1:]...)...)
	}
	return boxes
}

func average(box []uint32, tallies map[uint32]*colourTally) color.RGBA {
	var r, g, b, n int
	for _, c := range box {
		t := tallies[c]
		r, g, b, n = r+t.r, g+t.g, b+t.b, n+t.n
	}
	return color.RGBA{uint8((r + n/2) / n), uint8((g + n/2) / n), uint8((b + n/2) / n), 0xff}
}
