package state_test

import (
	"os"
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

func TestMain(m *testing.M) {
	os.Exit(helpers.RunIsolated(m))
}

func TestHomeIsolation(t *testing.T) {
	helpers.AssertIsolated(t)
}
