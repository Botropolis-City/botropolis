package claude

import "context"

// helperSlot holds the one Claude Code run the app may have going on
// its own behalf. Item 160: the budget for background helpers is one at
// a time across the usage probe and summaries, because each is a whole
// Claude Code of its own, at about 420 MiB.
var helperSlot = make(chan struct{}, 1)

// takeHelperSlot waits for the slot, and gives up when ctx ends.
// Waiting rather than skipping: a skipped scheduled probe would not be
// tried again for a whole interval, and a summary the person asked for
// should queue rather than fail.
func takeHelperSlot(ctx context.Context) (release func(), err error) {
	select {
	case helperSlot <- struct{}{}:
		return func() { <-helperSlot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
