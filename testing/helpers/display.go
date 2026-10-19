package helpers

import (
	"os"
	"os/exec"
	"testing"

	"botropolis.city/botropolis/pkg/xvfb"
)

// NeedXvfb skips a test that opens a virtual display when Xvfb is not
// installed, and fails it under CI, where a skip would read as green.
func NeedXvfb(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("Xvfb"); err == nil {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatal("Xvfb is not installed, and CI must open the city on a virtual display")
	}
	t.Skip("no Xvfb to open the city on a virtual display")
}

// Display starts a private virtual display of width by height for one
// test and stops it when the test ends. The server reports the display
// it took, so tests started together never share one (item 126).
func Display(t testing.TB, width, height int) *xvfb.Server {
	t.Helper()
	NeedXvfb(t)
	s, err := xvfb.Start(t.TempDir(), width, height)
	if err != nil {
		t.Fatalf("starting Xvfb: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
