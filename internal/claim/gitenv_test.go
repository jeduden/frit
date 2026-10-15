package claim

import (
	"os"
	"strconv"
	"testing"
)

// unpinnedCount is GIT_CONFIG_COUNT as the suite found it, before the
// signing pin was appended, so a test that lifts the pin drops the pin
// and nothing the environment itself supplied.
var unpinnedCount string

// TestMain pins commit.gpgsign off for every git the suite runs, the
// markers frit mints included: a mint now honors the setting, so a
// developer's global signing would otherwise sign — or prompt for — each
// fixture marker. The signing tests lift the pin for themselves.
func TestMain(m *testing.M) {
	unpinnedCount = pinSigningOff()
	os.Exit(m.Run())
}

// pinSigningOff appends commit.gpgsign=false to git's GIT_CONFIG_COUNT
// environment, at command-line precedence above any config file, after
// the entries the environment already carries — a sandbox's url
// rewrites or safe.directory stay in force — and returns the count it
// found.
func pinSigningOff() string {
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

	return n
}

// liftSigningPin drops TestMain's signing pin for one test, restoring
// the environment's own GIT_CONFIG_COUNT entries rather than none.
func liftSigningPin(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", unpinnedCount)
}
