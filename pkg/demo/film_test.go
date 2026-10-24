package demo

import (
	"fmt"
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

	t.Run("when a clip is recorded", func(t *testing.T) {
		shot := Shot{Name: "c", Record: &Recording{Seconds: 12, FPS: 30}}

		t.Run("it should record frames for that long at that rate", func(t *testing.T) {
			assert.Equal(t, append(base, "--window", "1920x1080", "--record", "/f", "--seconds", "12", "--fps", "30"),
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
