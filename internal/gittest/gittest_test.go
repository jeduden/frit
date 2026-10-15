package gittest

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guardEnv registers each GIT_CONFIG_* key PinSigningOff may write with
// t.Setenv, so the test's cleanup restores them even though the pin
// itself writes through os.Setenv.
func guardEnv(t *testing.T, count string, keys ...string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", count)
	for _, k := range keys {
		t.Setenv(k, os.Getenv(k))
	}
}

// TestPinSigningOffAppendsAfterTheEnvironmentsEntries: entries a host
// already passes through GIT_CONFIG_COUNT stay where they are, and the
// pin takes the next slot.
func TestPinSigningOffAppendsAfterTheEnvironmentsEntries(t *testing.T) {
	guardEnv(t, "2", "GIT_CONFIG_KEY_2", "GIT_CONFIG_VALUE_2")
	t.Setenv("GIT_CONFIG_KEY_0", "safe.directory")
	t.Setenv("GIT_CONFIG_VALUE_0", "*")

	unpinned, err := PinSigningOff()

	require.NoError(t, err)
	assert.Equal(t, "2", unpinned)
	assert.Equal(t, "3", os.Getenv("GIT_CONFIG_COUNT"))
	assert.Equal(t, "commit.gpgsign", os.Getenv("GIT_CONFIG_KEY_2"))
	assert.Equal(t, "false", os.Getenv("GIT_CONFIG_VALUE_2"))
	assert.Equal(t, "safe.directory", os.Getenv("GIT_CONFIG_KEY_0"))
}

// TestPinSigningOffTakesTheFirstSlotInAnEmptyEnvironment: with no
// GIT_CONFIG_COUNT set, the pin is entry 0.
func TestPinSigningOffTakesTheFirstSlotInAnEmptyEnvironment(t *testing.T) {
	guardEnv(t, "", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0")

	unpinned, err := PinSigningOff()

	require.NoError(t, err)
	assert.Equal(t, "0", unpinned)
	assert.Equal(t, "1", os.Getenv("GIT_CONFIG_COUNT"))
	assert.Equal(t, "commit.gpgsign", os.Getenv("GIT_CONFIG_KEY_0"))
}

// TestPinSigningOffOutranksARepositorysOwnSetting: git itself reads the
// pin above a repository's commit.gpgsign=true.
func TestPinSigningOffOutranksARepositorysOwnSetting(t *testing.T) {
	guardEnv(t, "", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0")
	repo := t.TempDir()
	git := func(args ...string) string {
		out, err := exec.Command("git",
			append([]string{"-C", repo}, args...)...).CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)

		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "commit.gpgsign", "true")

	_, err := PinSigningOff()

	require.NoError(t, err)
	assert.Equal(t, "false", git("config", "--get", "commit.gpgsign"))
}

// TestPinSigningOffSurfacesASetenvFault: a variable the process cannot
// set is returned to the caller, not panicked on.
func TestPinSigningOffSurfacesASetenvFault(t *testing.T) {
	orig := setenv
	t.Cleanup(func() { setenv = orig })
	setenv = func(string, string) error { return errors.New("setenv refused") }

	_, err := PinSigningOff()

	require.Error(t, err)
}

// TestLiftSigningPinRestoresTheEnvironmentsCount: lifting the pin
// drops the pin's own entry and none of the host's.
func TestLiftSigningPinRestoresTheEnvironmentsCount(t *testing.T) {
	guardEnv(t, "2", "GIT_CONFIG_KEY_2", "GIT_CONFIG_VALUE_2")
	unpinned, err := PinSigningOff()
	require.NoError(t, err)

	t.Run("lifted", func(t *testing.T) {
		LiftSigningPin(t, unpinned)
		assert.Equal(t, "2", os.Getenv("GIT_CONFIG_COUNT"))
	})
	assert.Equal(t, "3", os.Getenv("GIT_CONFIG_COUNT"),
		"the lift ends with the test that asked for it")
}
