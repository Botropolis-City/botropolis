package render

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"io"
	"os"
	"slices"
	"sync/atomic"

	xdraw "golang.org/x/image/draw"
)

// A shared GIF (items 168 and 188): four seconds at thirty frames a
// second, one frame a tick of the city's clock, drawn at 1080p. It needs
// no encoder but the standard library's.
const (
	gifSeconds = 4
	gifFPS     = 30
	gifWidth   = 1920
	gifHeight  = 1080
	// gifShade is the most a channel may move from its true value, of
	// 255: what median cut into 255 colours, over five bits a channel,
	// with each colour mapped to its nearest, costs a gradient of
	// thousands of colours that every frame shares. Measured at 15, not
	// derived; frames whose colours differ from each other share the one
	// palette between them, and cost more (42 for three unlike gradients).
	gifShade = 16
)

// gifWriter keeps a GIF's frames off the game loop, and writes the file
// once the last has come. The whole clip shares one palette, built from
// every frame's colours, so a pixel that does not change keeps its
// index from frame to frame: nothing that stands still can shimmer.
// Until the palette exists, each frame is held as its colours at five
// bits a channel, against the frame before and compressed, which a city
// that mostly stands still makes small.
type gifWriter struct {
	frames chan image.Image
	done   chan error
	// discard is set when the recording is abandoned: nothing is written.
	discard atomic.Bool
}

// gifQueue bounds the frames waiting to be encoded, so a slow machine
// makes the loop wait rather than hold every frame's pixels.
const gifQueue = 8

func newGIFWriter(path string, fps int) *gifWriter {
	w := &gifWriter{frames: make(chan image.Image, gifQueue), done: make(chan error, 1)}
	go func() {
		var clip gifClip
		for img := range w.frames {
			if w.discard.Load() {
				continue
			}
			if err := clip.add(img); err != nil {
				w.discard.Store(true)
				w.done <- err
				for range w.frames {
				}
				return
			}
		}
		if len(clip.held) == 0 || w.discard.Load() {
			w.done <- nil
			return
		}
		anim, err := clip.encode(fps)
		if err != nil {
			w.done <- err
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

// gifKeys is how many colours there are at five bits a channel.
const gifKeys = 1 << 15

// gifClip is a GIF's frames before its palette exists: each frame's
// colour keys, XORed with the frame before's and compressed, and the
// tally of every key across them all.
type gifClip struct {
	size  image.Rectangle
	held  [][]byte
	last  []uint16
	count [gifKeys]int
	sum   [gifKeys][3]int
}

// add keys, tallies and holds one frame; a frame of another size than
// the first is scaled to it.
func (c *gifClip) add(img image.Image) error {
	if c.held == nil {
		c.size = image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy())
		c.last = make([]uint16, c.size.Dx()*c.size.Dy())
	}
	rgba := asRGBA(img, c.size)
	delta := make([]uint16, len(c.last))
	w, h := c.size.Dx(), c.size.Dy()
	for y := range h {
		row := rgba.Pix[y*rgba.Stride:]
		for x := range w {
			r, g, b := row[4*x], row[4*x+1], row[4*x+2]
			k := gifKey(r, g, b)
			c.count[k]++
			c.sum[k][0] += int(r)
			c.sum[k][1] += int(g)
			c.sum[k][2] += int(b)
			i := y*w + x
			delta[i] = k ^ c.last[i]
			c.last[i] = k
		}
	}
	var buf bytes.Buffer
	z, err := flate.NewWriter(&buf, flate.BestSpeed)
	if err != nil {
		return err
	}
	if err := binary.Write(z, binary.LittleEndian, delta); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	c.held = append(c.held, buf.Bytes())
	return nil
}

func gifKey(r, g, b uint8) uint16 {
	return uint16(r>>3)<<10 | uint16(g>>3)<<5 | uint16(b>>3)
}

func asRGBA(img image.Image, size image.Rectangle) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect == size {
		return rgba
	}
	out := image.NewRGBA(size)
	xdraw.ApproxBiLinear.Scale(out, size, img, img.Bounds(), xdraw.Src, nil)
	return out
}

// encode builds the clip's one palette and its frames: the first whole,
// each after it cropped to the box that changed, with what did not
// change inside the box left transparent, over the frame before.
func (c *gifClip) encode(fps int) (*gif.GIF, error) {
	palette, index := c.palette()
	clear := uint8(len(palette) - 1)
	w, h := c.size.Dx(), c.size.Dy()
	anim := &gif.GIF{Config: image.Config{ColorModel: palette, Width: w, Height: h}}
	keys := make([]uint16, w*h)
	delta := make([]uint16, w*h)
	var shown []uint8
	for i, held := range c.held {
		if err := binary.Read(flate.NewReader(bytes.NewReader(held)), binary.LittleEndian, delta); err != nil && err != io.EOF {
			return nil, err
		}
		now := make([]uint8, w*h)
		for p, d := range delta {
			keys[p] ^= d
			now[p] = index[keys[p]]
		}
		var frame *image.Paletted
		if shown == nil {
			frame = &image.Paletted{Pix: now, Stride: w, Rect: c.size, Palette: palette}
		} else {
			frame = changed(shown, now, w, h, palette, clear)
		}
		shown = now
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, frameDelay(i, fps))
		anim.Disposal = append(anim.Disposal, gif.DisposalNone)
	}
	return anim, nil
}

// changed is the part of now that differs from shown: the box around
// every pixel that changed, transparent where a pixel inside it did not.
// A frame with no change is one transparent pixel.
func changed(shown, now []uint8, w, h int, palette color.Palette, clear uint8) *image.Paletted {
	box := image.Rectangle{}
	for y := range h {
		for x := range w {
			if shown[y*w+x] != now[y*w+x] {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if box.Empty() {
		box = image.Rect(0, 0, 1, 1)
	}
	out := image.NewPaletted(box, palette)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			v := now[y*w+x]
			if v == shown[y*w+x] {
				v = clear
			}
			out.Pix[out.PixOffset(x, y)] = v
		}
	}
	return out
}

// palette fits every key the clip used into 255 colours by median cut,
// and adds a transparent one last for what a frame leaves unchanged. The
// colours are split, again and again, at the median of the box with the
// widest spread, and each box's colour is the average of the true
// colours in it, so a clip with 255 colours or fewer keeps them exactly.
func (c *gifClip) palette() (color.Palette, *[gifKeys]uint8) {
	var used []uint16
	for k, n := range c.count {
		if n > 0 {
			used = append(used, uint16(k))
		}
	}
	boxes := c.medianCut(used, 255)
	palette := make(color.Palette, 0, len(boxes)+1)
	var index [gifKeys]uint8
	for _, box := range boxes {
		var r, g, b, n int
		for _, k := range box {
			r, g, b, n = r+c.sum[k][0], g+c.sum[k][1], b+c.sum[k][2], n+c.count[k]
		}
		palette = append(palette, color.RGBA{uint8((r + n/2) / n), uint8((g + n/2) / n), uint8((b + n/2) / n), 0xff})
	}
	for _, k := range used {
		index[k] = c.nearest(k, palette)
	}
	return append(palette, color.RGBA{}), &index
}

// nearest is the palette colour closest to a key's average colour, which
// is not always its own box's: a box's average sits at its weight, and a
// key at the box's edge can be nearer the next.
func (c *gifClip) nearest(k uint16, palette color.Palette) uint8 {
	n := c.count[k]
	r, g, b := (c.sum[k][0]+n/2)/n, (c.sum[k][1]+n/2)/n, (c.sum[k][2]+n/2)/n
	best, at := -1, 0
	for i, p := range palette {
		q := p.(color.RGBA)
		dr, dg, db := r-int(q.R), g-int(q.G), b-int(q.B)
		d := 2*dr*dr + 4*dg*dg + 3*db*db
		if best < 0 || d < best {
			best, at = d, i
		}
	}
	return uint8(at)
}

func keyChannel(k uint16, ch int) int { return int(k>>(10-5*ch)) & 31 }

func (c *gifClip) medianCut(keys []uint16, want int) [][]uint16 {
	if len(keys) <= want {
		boxes := make([][]uint16, len(keys))
		for i, k := range keys {
			boxes[i] = []uint16{k}
		}
		return boxes
	}
	boxes := [][]uint16{keys}
	for len(boxes) < want {
		widest, ch, spread := -1, 0, 0
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			for c := range 3 {
				lo, hi := 31, 0
				for _, k := range box {
					v := keyChannel(k, c)
					lo, hi = min(lo, v), max(hi, v)
				}
				if hi-lo > spread {
					widest, ch, spread = i, c, hi-lo
				}
			}
		}
		if widest < 0 {
			break
		}
		box := boxes[widest]
		slices.SortFunc(box, func(a, b uint16) int { return keyChannel(a, ch) - keyChannel(b, ch) })
		total := 0
		for _, k := range box {
			total += c.count[k]
		}
		cut, run := 1, 0
		for i, k := range box[:len(box)-1] {
			run += c.count[k]
			if run*2 >= total {
				cut = i + 1
				break
			}
		}
		boxes = append(boxes[:widest], append([][]uint16{box[:cut], box[cut:]}, boxes[widest+1:]...)...)
	}
	return boxes
}
