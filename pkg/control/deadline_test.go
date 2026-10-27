package control_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/pkg/control"
)

func TestDeadlines(t *testing.T) {
	for _, verb := range []struct {
		name    string
		timeout time.Duration
		call    func(c *control.Control) error
	}{
		{"new", 2 * time.Minute, func(c *control.Control) error {
			_, err := c.New(context.Background(), dir, "")
			return err
		}},
		{"resume", 2 * time.Minute, func(c *control.Control) error {
			_, err := c.Resume(context.Background(), dir, sessionID)
			return err
		}},
		{"fork", 2 * time.Minute, func(c *control.Control) error {
			_, err := c.Fork(context.Background(), dir, sessionID)
			return err
		}},
		{"stop", 30 * time.Second, func(c *control.Control) error { return c.Stop(context.Background(), jobID) }},
		{"rm", 30 * time.Second, func(c *control.Control) error { return c.Remove(context.Background(), jobID) }},
		{"agents", 20 * time.Second, func(c *control.Control) error {
			_, err := c.Agents(context.Background(), true)
			return err
		}},
	} {
		t.Run("when claude "+verb.name+" runs", func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{stdout: backgrounded, agents: `[]`}
			start := time.Now()
			_ = verb.call(newControl(runner))
			require.NotEmpty(t, runner.deadlines)

			t.Run("it should hand claude a deadline of "+verb.timeout.String(), func(t *testing.T) {
				assert.WithinDuration(t, start.Add(verb.timeout), runner.deadlines[0], time.Second)
			})
		})

		t.Run("when claude "+verb.name+" does not answer", func(t *testing.T) {
			t.Parallel()
			c := newControl(&fakeRunner{hang: true})
			control.ShortenTimeouts(c, 20*time.Millisecond)
			err := verb.call(c)

			t.Run("it should say claude did not answer in time", func(t *testing.T) {
				assert.ErrorContains(t, err, "claude "+verb.name+" did not answer within 20ms")
			})
		})
	}
}
