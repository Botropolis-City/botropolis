//go:build devtools

package cli_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/cli"
)

func TestHoverFlag(t *testing.T) {
	parse := func(t *testing.T, args ...string) *cobra.Command {
		t.Helper()
		cmd := &cobra.Command{Use: "city", RunE: func(*cobra.Command, []string) error { return nil }}
		cli.AddHoverFlag(cmd.Flags())
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		return cmd
	}

	t.Run("when a point is given", func(t *testing.T) {
		cmd := parse(t, "--hover", "640,360")
		x, y, ok := cli.Hover(cmd)

		t.Run("it should be taken", func(t *testing.T) {
			require.True(t, ok)
		})

		t.Run("it should read the x", func(t *testing.T) {
			assert.Equal(t, 640.0, x)
		})

		t.Run("it should read the y", func(t *testing.T) {
			assert.Equal(t, 360.0, y)
		})
	})

	t.Run("when the flag is left off", func(t *testing.T) {
		t.Run("it should say so, because 0,0 is a real place to point", func(t *testing.T) {
			_, _, ok := cli.Hover(parse(t))
			assert.False(t, ok)
		})
	})

	t.Run("when the point does not parse", func(t *testing.T) {
		t.Run("it should be ignored rather than guessed at", func(t *testing.T) {
			_, _, ok := cli.Hover(parse(t, "--hover", "the middle"))
			assert.False(t, ok)
		})
	})
}

func TestWindowFlag(t *testing.T) {
	parse := func(t *testing.T, args ...string) *cobra.Command {
		t.Helper()
		cmd := &cobra.Command{Use: "city", RunE: func(*cobra.Command, []string) error { return nil }}
		cli.AddWindowFlag(cmd.Flags())
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		return cmd
	}

	t.Run("when a size is given", func(t *testing.T) {
		w, h, err := cli.Window(parse(t, "--window", "1920x1080"))
		require.NoError(t, err)

		t.Run("it should read the width and height", func(t *testing.T) {
			assert.Equal(t, [2]int{1920, 1080}, [2]int{w, h})
		})
	})

	t.Run("when the flag is left off", func(t *testing.T) {
		w, h, err := cli.Window(parse(t))
		require.NoError(t, err)

		t.Run("it should leave the size to the window's default", func(t *testing.T) {
			assert.Equal(t, [2]int{0, 0}, [2]int{w, h})
		})
	})

	for _, raw := range []string{"1920", "wide", "0x1080", "1920x-1"} {
		t.Run("when the size is "+raw, func(t *testing.T) {
			_, _, err := cli.Window(parse(t, "--window", raw))

			t.Run("it should refuse it rather than open some other size", func(t *testing.T) {
				assert.Error(t, err)
			})
		})
	}
}

func TestFPSFlag(t *testing.T) {
	parse := func(t *testing.T, args ...string) *cobra.Command {
		t.Helper()
		cmd := &cobra.Command{Use: "city", RunE: func(*cobra.Command, []string) error { return nil }}
		cli.AddRecordFlags(cmd.Flags())
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		return cmd
	}

	t.Run("when the flag is left off", func(t *testing.T) {
		fps, err := cli.FPS(parse(t))
		require.NoError(t, err)

		t.Run("it should record at ten frames a second, as it always has", func(t *testing.T) {
			assert.Equal(t, 10, fps)
		})
	})

	t.Run("when thirty is asked for", func(t *testing.T) {
		fps, err := cli.FPS(parse(t, "--fps", "30"))
		require.NoError(t, err)

		t.Run("it should take it", func(t *testing.T) {
			assert.Equal(t, 30, fps)
		})
	})

	for _, raw := range []string{"7", "0", "60"} {
		t.Run("when "+raw+" is asked for", func(t *testing.T) {
			_, err := cli.FPS(parse(t, "--fps", raw))

			t.Run("it should refuse, since it does not divide the 30 ticks a second", func(t *testing.T) {
				assert.Error(t, err)
			})
		})
	}
}
