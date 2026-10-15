package main

import (
	"os"
	"strconv"
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

// pinSigningOff appends commit.gpgsign=false to git's GIT_CONFIG_COUNT
// environment, at command-line precedence above any config file, after
// the entries the environment already carries — a sandbox's url
// rewrites or safe.directory stay in force.
func pinSigningOff() {
	prior, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	n := strconv.Itoa(prior)
	for k, v := range map[string]string{
		"GIT_CONFIG_COUNT":      strconv.Itoa(prior + 1),
		"GIT_CONFIG_KEY_" + n:   "commit.gpgsign",
		"GIT_CONFIG_VALUE_" + n: "false",
	} {
		if err := os.Setenv(k, v); err != nil {
			panic(err)
		}
	}
}
