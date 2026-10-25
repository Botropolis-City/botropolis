package helpers_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"botropolis.city/botropolis/testing/helpers"
)

func writeHome(t *testing.T, history []helpers.HistoryPrompt, layout string) string {
	return helpers.NewHome(t).WriteHistory(history, layout).Path
}

func hashTree(t *testing.T, root string) string {
	h := sha256.New()
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(h, "%s %v\n", rel, info.Mode())
		if d.Type().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%d\n", len(content))
			h.Write(content)
		}
		return nil
	}))
	return hex.EncodeToString(h.Sum(nil))
}

// The cost tests' homes, hashed before item 248 moved their generator
// into pkg/demo, so the move is held to writing every byte as before.
func TestHistoryHomes(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	homes := []struct {
		scenario string
		history  []helpers.HistoryPrompt
		layout   string
		want     string
	}{
		{scenario: "when the cost tests' 2,000 small sessions are written from seed 198",
			history: helpers.ManySessions(198, 2000, now), layout: "2006-01-02T15:04:05.000Z",
			want: "11a75de9f692d17d85018367ee07e2391e8db0fe7d29d2256c260d7373a7edd5"},
		{scenario: "when the cost tests' 100 sessions are written from seed 158",
			history: helpers.SyntheticHistory(158, 100, 40, now.Add(-20*24*time.Hour)), layout: "2006-01-02T15:04:05.000Z",
			want: "a3cc41c0d5abe7b211ae973ce9823c6e4fccb126d0f2d540676ea17aa5a2559c"},
		{scenario: "when the daemon test's 2,000 small sessions are written with nanosecond times",
			history: helpers.ManySessions(198, 2000, now), layout: time.RFC3339Nano,
			want: "db4c3234cd7ca53ec2e7fe5e8e87989817fdcaa8c310716ea4f2c58a2463bf34"},
	}
	for _, h := range homes {
		t.Run(h.scenario, func(t *testing.T) {
			got := hashTree(t, writeHome(t, h.history, h.layout))

			t.Run("it should write the home it wrote before the move", func(t *testing.T) {
				assert.Equal(t, h.want, got)
			})
		})
	}
}
