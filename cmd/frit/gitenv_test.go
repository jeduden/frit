package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/jeduden/frit/internal/gittest"
)

// TestMain pins commit.gpgsign off for every git the suite runs, the
// markers a claim mints included: a mint honors the setting, so a
// developer's global signing would otherwise sign — or prompt for — each
// fixture marker.
func TestMain(m *testing.M) {
	if _, err := gittest.PinSigningOff(); err != nil {
		fmt.Fprintln(os.Stderr, "pin commit.gpgsign off:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
