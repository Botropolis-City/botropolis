package proc

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStat(t *testing.T) {
	stat := func(name string, fields int) []byte {
		after := make([]string, fields)
		for i := range after {
			after[i] = fmt.Sprint(i + 3)
		}
		return []byte(fmt.Sprintf("4242 (%s) %s\n", name, strings.Join(after, " ")))
	}

	t.Run("when the process's name holds spaces and parentheses", func(t *testing.T) {
		started, _ := parseStat(stat("a) (b c", 50))

		t.Run("it should read field 22 counted from after the name", func(t *testing.T) {
			assert.Equal(t, uint64(22), started)
		})
	})

	t.Run("when the stat is truncated", func(t *testing.T) {
		_, ok := parseStat(stat("sleep", 19))

		t.Run("it should not know", func(t *testing.T) {
			assert.False(t, ok)
		})
	})
}
