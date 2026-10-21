package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"botropolis.city/botropolis/pkg/render"
	"botropolis.city/botropolis/pkg/xvfb"
)

// sheetDisplayEnv marks the child sheetCommand runs on its own virtual
// display, so the child draws instead of starting another.
const sheetDisplayEnv = "BOTROPOLIS_SHEET_DISPLAY"

func sheetCommand() *cobra.Command {
	out, zoom, check, looks := "", 1.0, false, false
	framesHome, hour, frames, dumpDir, writeDir := "", 14, 30, "", ""
	cmd := &cobra.Command{
		Use:   "sheet",
		Short: "Draw every building piece, lived in and boarded up, by day and by night, to a PNG",
		Long: "Draws each district kind at every tier and its ruins four ways, with the\n" +
			"map's own drawing, on a private virtual display (Xvfb). Item 166.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out == "" && !check && !looks && framesHome == "" {
				return fmt.Errorf("--out is required")
			}
			if os.Getenv(sheetDisplayEnv) != "" {
				if framesHome != "" {
					now := time.Now()
					at := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.Local)
					problems, err := render.CheckFrames(framesHome, at, frames, dumpDir, writeDir)
					if err != nil {
						return err
					}
					for _, p := range problems {
						fmt.Fprintln(cmd.ErrOrStderr(), p)
					}
					if len(problems) > 0 {
						return fmt.Errorf("%d frames drawn wrongly through the standing city", len(problems))
					}
					return nil
				}
				if looks {
					problems, err := render.CheckLooks(zoom)
					if err != nil {
						return err
					}
					for _, p := range problems {
						fmt.Fprintln(cmd.ErrOrStderr(), p)
					}
					if len(problems) > 0 {
						return fmt.Errorf("%d building looks drawn wrongly through the cache", len(problems))
					}
					return nil
				}
				if check {
					problems, err := render.CheckLights(zoom)
					if err != nil {
						return err
					}
					for _, p := range problems {
						fmt.Fprintln(cmd.ErrOrStderr(), p)
					}
					if len(problems) > 0 {
						return fmt.Errorf("%d buildings lit wrongly at night", len(problems))
					}
					return nil
				}
				return render.Sheet(out, zoom)
			}
			dir, err := os.MkdirTemp("", "botropolis-sheet-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			server, err := xvfb.Start(dir, 640, 480)
			if err != nil {
				return err
			}
			defer func() { _ = server.Close() }()
			child := exec.CommandContext(cmd.Context(), os.Args[0], os.Args[1:]...)
			child.Stdout, child.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
			var env []string
			for _, kv := range os.Environ() {
				switch key, _, _ := strings.Cut(kv, "="); key {
				case "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY":
				default:
					env = append(env, kv)
				}
			}
			child.Env = append(append(env, server.Env()...), sheetDisplayEnv+"=1")
			if err := child.Run(); err != nil {
				return err
			}
			switch {
			case framesHome != "":
				fmt.Fprintln(cmd.OutOrStdout(), "every frame drawn through the standing city matches the city drawn straight")
			case looks:
				fmt.Fprintln(cmd.OutOrStdout(), "every building's cached look matches the building drawn directly")
			case check:
				fmt.Fprintln(cmd.OutOrStdout(), "every lit building lights its own panes and nothing else")
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", out, "PNG to write")
	cmd.Flags().Float64Var(&zoom, "zoom", zoom, "camera zoom to draw at; 1 is the map's")
	cmd.Flags().BoolVar(&check, "check-lights", check, "draw no sheet; check every building's night lights fall on its panes")
	cmd.Flags().BoolVar(&looks, "check-looks", looks, "draw no sheet; check every building drawn through the look cache matches it drawn directly")
	cmd.Flags().StringVar(&framesHome, "check-frames", framesHome, "draw no sheet; check the city staged in this home drawn through the standing city matches it drawn straight, frame by frame")
	cmd.Flags().IntVar(&hour, "hour", hour, "with --check-frames, the local hour the city is shown at")
	cmd.Flags().IntVar(&frames, "frames", frames, "with --check-frames, how many frames to check each way the city is shown")
	cmd.Flags().StringVar(&dumpDir, "dump", dumpDir, "with --check-frames, a directory to write both drawings of each frame that differs to")
	cmd.Flags().StringVar(&writeDir, "write", writeDir, "with --check-frames, a directory to write both drawings of each way's last frame to")
	return cmd
}
