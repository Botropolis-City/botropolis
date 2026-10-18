package helpers

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Tool returns the path to a script under tools/ for a test to run. A script
// runs in a subprocess, whose reads go test's cache cannot see, so Tool opens
// the script and the tools/ directory, which puts every file there, and any
// sibling the script runs, in the cache's key (item 117).
func Tool(t testing.TB, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the helpers package on disk")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "tools")
	if _, err := os.ReadDir(dir); err != nil {
		t.Fatalf("reading tools/: %v", err)
	}
	path := filepath.Join(dir, name)
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("reading tools/%s: %v", name, err)
	}
	return path
}
