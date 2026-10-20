//go:build !devtools

package cli

import "github.com/spf13/pflag"

// addFrameFlags registers nothing in a release build: the frame tooling
// is only in builds with the devtools tag. Item 167.
func addFrameFlags(*pflag.FlagSet, bool) {}
