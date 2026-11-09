package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/jeduden/frit/internal/gittest"
)

// TestMain unsets the holder's own inputs, so a suite run inside a
// Claude Code cloud session mints markers under the hostname the
// fixtures expect, as it does everywhere else; a test that wants a
// holder sets one itself. It also pins commit.gpgsign off for every git the suite runs, the
// markers a claim mints included: a mint honors the setting, so a
// developer's global signing would otherwise sign — or prompt for — each
// fixture marker.
func TestMain(m *testing.M) {
	for _, name := range []string{"FRIT_HOLDER", cloudSessionEnv} {
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintln(os.Stderr, "unset", name+":", err)
			os.Exit(1)
		}
	}
	if _, err := gittest.PinSigningOff(); err != nil {
		fmt.Fprintln(os.Stderr, "pin commit.gpgsign off:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
