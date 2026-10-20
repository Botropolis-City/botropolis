package commands_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"botropolis.city/botropolis/pkg/commands"
)

func TestDaemonEnsure(t *testing.T) {
	type outcome struct {
		ran []string
		out string
		err error
	}
	ensure := func(goos string, onPath []string, running, dryRun bool, failure error) outcome {
		var o outcome
		var out bytes.Buffer
		d := commands.Daemon{
			GOOS:    goos,
			OnPath:  func(name string) bool { return strings.Contains(" "+strings.Join(onPath, " ")+" ", " "+name+" ") },
			Running: func() bool { return running },
			Run: func(argv []string) error {
				o.ran = argv
				return failure
			},
		}
		o.err = d.Ensure(&out, dryRun)
		o.out = out.String()
		return o
	}

	for _, c := range []struct {
		scenario string
		goos     string
		onPath   []string
		want     []string
	}{
		{"when it is Linux with systemctl", "linux", []string{"systemctl"}, []string{"systemctl", "--user", "enable", "--now", "botropolisd"}},
		{"when it is macOS with brew", "darwin", []string{"brew"}, []string{"brew", "services", "start", "botropolis"}},
	} {
		t.Run(c.scenario, func(t *testing.T) {
			t.Run("and the daemon is not running", func(t *testing.T) {
				o := ensure(c.goos, c.onPath, false, false, nil)

				t.Run("it should run the platform's command", func(t *testing.T) {
					assert.Equal(t, c.want, o.ran)
				})

				t.Run("it should print exactly what it ran", func(t *testing.T) {
					assert.Equal(t, "started the daemon: "+strings.Join(c.want, " ")+"\n", o.out)
				})
			})

			t.Run("and the daemon is running", func(t *testing.T) {
				o := ensure(c.goos, c.onPath, true, false, nil)

				t.Run("it should run nothing", func(t *testing.T) {
					assert.Nil(t, o.ran)
				})

				t.Run("it should say it is running", func(t *testing.T) {
					assert.Equal(t, "the daemon is running\n", o.out)
				})
			})

			t.Run("and it is a dry run", func(t *testing.T) {
				o := ensure(c.goos, c.onPath, false, true, nil)

				t.Run("it should run nothing", func(t *testing.T) {
					assert.Nil(t, o.ran)
				})

				t.Run("it should print what it would run", func(t *testing.T) {
					assert.Equal(t, "would start the daemon: "+strings.Join(c.want, " ")+"\n", o.out)
				})
			})

			t.Run("and the command fails", func(t *testing.T) {
				o := ensure(c.goos, c.onPath, false, false, errors.New("exit status 1"))

				t.Run("it should name the command in its error", func(t *testing.T) {
					assert.EqualError(t, o.err, strings.Join(c.want, " ")+": exit status 1")
				})
			})
		})
	}

	t.Run("when neither systemctl nor brew is on the path", func(t *testing.T) {
		o := ensure("linux", nil, false, false, nil)

		t.Run("it should run nothing", func(t *testing.T) {
			assert.Nil(t, o.ran)
		})

		t.Run("it should say how to run botropolisd", func(t *testing.T) {
			assert.Equal(t, "the daemon is not running: start botropolisd with your service manager, or run it in a terminal\n", o.out)
		})
	})
}
