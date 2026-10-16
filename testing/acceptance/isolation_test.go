package acceptance_test

import (
	"testing"

	"botropolis.city/botropolis/testing/helpers"
)

func TestHomeIsolation(t *testing.T) {
	helpers.AssertIsolated(t)
}
