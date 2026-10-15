package claim

import (
	"fmt"
	"os"
	"testing"

	"github.com/jeduden/frit/internal/gittest"
)

// unpinnedCount is GIT_CONFIG_COUNT as the suite found it, before the
// signing pin was appended, so a test that lifts the pin drops the pin
// and nothing the environment itself supplied.
var unpinnedCount string

// TestMain pins commit.gpgsign off for every git the suite runs, the
// markers frit mints included: a mint honors the setting, so a
// developer's global signing would otherwise sign — or prompt for — each
// fixture marker. The signing tests lift the pin for themselves.
func TestMain(m *testing.M) {
	var err error
	if unpinnedCount, err = gittest.PinSigningOff(); err != nil {
		fmt.Fprintln(os.Stderr, "pin commit.gpgsign off:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// liftSigningPin drops TestMain's signing pin for one test, restoring
// the environment's own GIT_CONFIG_COUNT entries rather than none.
func liftSigningPin(t *testing.T) {
	t.Helper()
	gittest.LiftSigningPin(t, unpinnedCount)
}
