package demo

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/script"
)

func ptr[T any](v T) *T { return &v }

func TestCityArgs(t *testing.T) {
	base := []string{"city", "--headless", "--home", "/h", "--socket", "/nonexistent/botropolis.sock"}

	t.Run("when a still is shot with nothing but a name", func(t *testing.T) {
		t.Run("it should screenshot at the default window", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--screenshot", "/o/still.png"),
				cityArgs(Shot{Name: "still"}, "/h", "/o/still.png", ""))
		})
	})

	t.Run("when a still asks for keys, a hover point and a look", func(t *testing.T) {
		shot := Shot{Name: "s", Window: "1280x720", Keys: []string{"equal", "b"}, Hover: "640,360",
			Detail: "plain", Signage: "plates", Scale: 1.5}

		t.Run("it should pass each one through", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1280x720", "--keys", "equal,b", "--hover", "640,360",
				"--detail", "plain", "--signage", "plates", "--render_scale", "1.5",
				"--screenshot", "/o/s.png"),
				cityArgs(shot, "/h", "/o/s.png", ""))
		})
	})

	// Item 109: a shot can film every district as one type.
	t.Run("when a still names a district type", func(t *testing.T) {
		t.Run("it should pass it through", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--district_type", "office", "--screenshot", "/o/s.png"),
				cityArgs(Shot{Name: "s", DistrictType: "office"}, "/h", "/o/s.png", ""))
		})
	})

	// Item 85: the hero is filmed with the group lines hidden, through
	// the setting, so it shows the city rather than the wiring.
	for _, tc := range []struct {
		scenario string
		lines    *bool
		want     []string
	}{
		{"hides the group lines", ptr(false), []string{"--group_lines=false"}},
		{"shows the group lines", ptr(true), []string{"--group_lines=true"}},
		{"says nothing of the group lines", nil, nil},
	} {
		t.Run("when a still "+tc.scenario, func(t *testing.T) {
			args := cityArgs(Shot{Name: "s", GroupLines: tc.lines}, "/h", "/o/s.png", "")
			var got []string
			for _, a := range args {
				if strings.HasPrefix(a, "--group_lines") {
					got = append(got, a)
				}
			}

			t.Run("it should pass the setting through only when the shot sets it", func(t *testing.T) {
				assert.Equal(t, tc.want, got)
			})
		})
	}

	t.Run("when a still asks for a view and keys of its own", func(t *testing.T) {
		args := cityArgs(Shot{Name: "s", View: "models", Keys: []string{"h"}}, "/h", "/o/s.png", "")

		t.Run("it should press the view's keys before the shot's", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--keys", "Digit3,Digit3,h", "--screenshot", "/o/s.png"), args)
		})
	})

	t.Run("when a clip is recorded", func(t *testing.T) {
		shot := Shot{Name: "c", Record: &Recording{Seconds: 12, FPS: 30}}

		t.Run("it should record frames for that long at that rate", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--record", "/f", "--seconds", "12", "--fps", "30"),
				cityArgs(shot, "/h", "", "/f"))
		})
	})

	// Item 76: a loop records a lead for the fit to settle, and its
	// crossfade, beyond its own seconds.
	t.Run("when a looping clip is recorded", func(t *testing.T) {
		shot := Shot{Name: "loop", Record: &Recording{Seconds: 15, FPS: 30}, Loop: &Loop{Crossfade: 1}}

		t.Run("it should record the lead, the seconds and the crossfade", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--record", "/f", "--seconds", "17", "--fps", "30"),
				cityArgs(shot, "/h", "", "/f"))
		})
	})

	t.Run("when a clip has a script", func(t *testing.T) {
		shot := Shot{Name: "c", Record: &Recording{Seconds: 4}, Script: []script.Cue{{At: 1, Click: true}}}

		t.Run("it should hand the city the script written beside the frames", func(t *testing.T) {
			assert.Equal(t, []string{"--script", "/work/script.json"}, cityArgs(shot, "/h", "", "/work/frames")[8:10])
		})
	})

	t.Run("when the daemon is running for a timeline", func(t *testing.T) {
		args := cityArgs(Shot{Name: "c"}, "/h", "/o/c.png", "", withSocket("/run/d.sock"))

		t.Run("it should read from it rather than the files", func(t *testing.T) {
			assert.Equal(t, []string{"--socket", "/run/d.sock"}, args[4:6])
		})
	})
}

func TestEncodeArgs(t *testing.T) {
	for format, want := range map[string][]string{
		"mp4":  {"-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20", "-movflags", "+faststart"},
		"webm": {"-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", "-crf", "34", "-b:v", "0"},
	} {
		t.Run("when encoding "+format, func(t *testing.T) {
			args, err := encodeArgs(format, "/f", 30, "/o/c."+format)
			require.NoError(t, err)

			t.Run("it should use a codec every browser plays", func(t *testing.T) {
				assert.Equal(t, append(append([]string{"-loglevel", "error", "-y", "-framerate", "30", "-i", "/f/frame-%05d.png"}, want...), "/o/c."+format), args)
			})
		})
	}

	t.Run("when encoding at a given CRF", func(t *testing.T) {
		args, err := encodeArgs("mp4", "/f", 30, "/o/c.mp4", withCRF(26))
		require.NoError(t, err)

		t.Run("it should use it", func(t *testing.T) {
			assert.Equal(t, []string{"-crf", "26"}, args[11:13])
		})
	})

	t.Run("when encoding a 4:5 crop", func(t *testing.T) {
		args, err := encodeArgs("webm", "/f", 30, "/o/c-4x5.webm", withCrop("4:5"))
		require.NoError(t, err)

		t.Run("it should crop the middle of the frame to even dimensions", func(t *testing.T) {
			assert.Equal(t, []string{"-vf", "crop=trunc(ih*4/5/2)*2:ih"}, args[7:9])
		})
	})

	t.Run("when encoding a format it does not know", func(t *testing.T) {
		_, err := encodeArgs("avi", "/f", 30, "/o/c.avi")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestClockZone(t *testing.T) {
	now := time.Date(2026, 10, 7, 21, 39, 0, 0, time.UTC)

	for _, want := range []int{0, 6, 9, 14, 21, 23} {
		t.Run(fmt.Sprintf("when the scenario asks for %02d:00", want), func(t *testing.T) {
			tz, err := clockZone(now, fmt.Sprintf("%02d:00", want))
			require.NoError(t, err)
			zone, err := time.LoadLocation(tz)
			require.NoError(t, err)

			t.Run("it should name a zone whose local hour is that hour", func(t *testing.T) {
				assert.Equal(t, want, now.In(zone).Hour())
			})
		})
	}

	t.Run("when the clock does not parse", func(t *testing.T) {
		_, err := clockZone(now, "teatime")

		t.Run("it should refuse", func(t *testing.T) {
			assert.Error(t, err)
		})
	})
}

func TestFilmHold(t *testing.T) {
	const slowestFilmMeasured = time.Hour + 39*time.Minute

	t.Run("when a shot's stand-ins are spawned", func(t *testing.T) {
		t.Run("it should hold them longer than any film takes (item 187)", func(t *testing.T) {
			assert.Greater(t, filmHold, slowestFilmMeasured)
		})
	})
}

// Item 76: a clip's poster used to be whatever frame a third of the way
// in landed on, which on timelapse/day was the pointer mid-hover.
func TestPosterFrame(t *testing.T) {
	const frames, fps = 90, 30

	for _, tc := range []struct {
		when, should string
		at           *float64
		want         int
	}{
		{"sets no poster_at", "take the frame a third of the way in", nil, 30},
		{"sets poster_at inside the clip", "take the frame at that second", ptr(1.5), 45},
		{"sets poster_at between two frames", "take the nearer", ptr(1.49), 45},
		{"sets poster_at at the clip's last moment", "take the last frame rather than one past it", ptr(3.0), 89},
	} {
		t.Run("when a clip "+tc.when, func(t *testing.T) {
			t.Run("it should "+tc.should, func(t *testing.T) {
				assert.Equal(t, tc.want, posterFrame(frames, fps, tc.at))
			})
		})
	}
}

// fakeEncoder makes files that shrink as the CRF rises, as a real
// encoder's do.
type fakeEncoder struct {
	sizeAt func(crf int) int64
	tried  []int
}

func (e *fakeEncoder) encode(crf int) (int64, error) {
	e.tried = append(e.tried, crf)
	return e.sizeAt(crf), nil
}

func TestEncodeWithin(t *testing.T) {
	const mb = 1 << 20
	shrinking := func(crf int) int64 { return int64(10-(crf-20)/2) * mb }

	t.Run("when the first encode fits the budget", func(t *testing.T) {
		e := &fakeEncoder{sizeAt: func(int) int64 { return mb }}
		require.NoError(t, encodeWithin("/o/c.mp4", 20, 3*mb, e.encode))

		t.Run("it should encode once", func(t *testing.T) {
			assert.Equal(t, []int{20}, e.tried)
		})
	})

	t.Run("when it does not fit", func(t *testing.T) {
		e := &fakeEncoder{sizeAt: shrinking}
		require.NoError(t, encodeWithin("/o/c.mp4", 20, 7*mb, e.encode))

		t.Run("it should re-encode at higher CRFs until it fits", func(t *testing.T) {
			assert.Equal(t, []int{20, 22, 24, 26}, e.tried)
		})
	})

	t.Run("when nothing up to the ceiling fits", func(t *testing.T) {
		e := &fakeEncoder{sizeAt: func(int) int64 { return 9 * mb }}
		err := encodeWithin("/o/c.mp4", 20, 3*mb, e.encode)

		for _, want := range []string{"/o/c.mp4", "9.0 MB", "3.0 MB", fmt.Sprintf("CRF %d", 20+maxCRFRise)} {
			t.Run("it should fail, saying "+want, func(t *testing.T) {
				assert.ErrorContains(t, err, want)
			})
		}
	})
}

func TestCropPoster(t *testing.T) {
	src := filepath.Join(t.TempDir(), "p.png")
	img := image.NewNRGBA(image.Rect(0, 0, 1920, 1080))
	for x := 528; x < 528+864; x++ {
		img.Set(x, 0, color.NRGBA{R: 255, A: 255})
	}
	f, err := os.Create(src)
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, img))
	require.NoError(t, f.Close())
	dst := filepath.Join(t.TempDir(), "p-4x5.png")
	require.NoError(t, cropPoster(src, dst, "4:5"))
	out, err := os.Open(dst)
	require.NoError(t, err)
	defer func() { _ = out.Close() }()
	got, err := png.Decode(out)
	require.NoError(t, err)

	t.Run("when a 1920x1080 poster is cropped to 4:5", func(t *testing.T) {
		t.Run("it should be 864x1080", func(t *testing.T) {
			assert.Equal(t, image.Pt(864, 1080), got.Bounds().Size())
		})

		t.Run("it should come from the middle", func(t *testing.T) {
			left, _, _, _ := got.At(got.Bounds().Min.X, 0).RGBA()
			right, _, _, _ := got.At(got.Bounds().Max.X-1, 0).RGBA()
			assert.Equal(t, []uint32{0xffff, 0xffff}, []uint32{left, right})
		})
	})
}
