// Package gittest holds the git environment every suite that runs real
// git shares, so the packages that need it keep one copy rather than
// one each that can drift.
package gittest

import (
	"os"
	"strconv"
	"testing"
)

// setenv is os.Setenv behind a seam, so a test can force the fault
// PinSigningOff must return rather than panic on.
var setenv = os.Setenv

// PinSigningOff appends commit.gpgsign=false to git's GIT_CONFIG_COUNT
// environment, at command-line precedence above any config file, so a
// developer's global signing does not sign — or prompt for — each
// fixture marker a mint writes. It goes after the entries the
// environment already carries, so a sandbox's url rewrites or
// safe.directory stay in force, and returns the count it found for
// LiftSigningPin.
func PinSigningOff() (string, error) {
	prior, _ := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT"))
	n := strconv.Itoa(prior)
	for _, kv := range [][2]string{
		{"GIT_CONFIG_KEY_" + n, "commit.gpgsign"},
		{"GIT_CONFIG_VALUE_" + n, "false"},
		{"GIT_CONFIG_COUNT", strconv.Itoa(prior + 1)},
	} {
		if err := setenv(kv[0], kv[1]); err != nil {
			return "", err
		}
	}

	return n, nil
}

// LiftSigningPin drops PinSigningOff's entry for one test, restoring
// the environment's own GIT_CONFIG_COUNT — unpinned, as PinSigningOff
// returned it — rather than none.
func LiftSigningPin(t testing.TB, unpinned string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", unpinned)
}
