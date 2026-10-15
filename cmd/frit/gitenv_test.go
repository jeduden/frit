package main

import (
	"os"
	"testing"
)

// TestMain pins commit.gpgsign off for every git the suite runs, the
// markers a claim mints included: a mint honors the setting, so a
// developer's global signing would otherwise sign — or prompt for — each
// fixture marker.
func TestMain(m *testing.M) {
	pinSigningOff()
	os.Exit(m.Run())
}

// pinSigningOff sets commit.gpgsign=false at command-line precedence
// through git's GIT_CONFIG_COUNT environment, above any config file.
func pinSigningOff() {
	for k, v := range map[string]string{
		"GIT_CONFIG_COUNT":   "1",
		"GIT_CONFIG_KEY_0":   "commit.gpgsign",
		"GIT_CONFIG_VALUE_0": "false",
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
}
