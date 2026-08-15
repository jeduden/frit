package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env builds a getenv stand-in from a map, so a test states exactly
// the environment it depends on.
func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestPathsPrefersExplicitOverrideFirst(t *testing.T) {
	got := Paths(env(map[string]string{
		"FRIT_CONFIG":     "/etc/frit.yml",
		"XDG_CONFIG_HOME": "/home/u/.config",
	}), "/work")

	require.Len(t, got, 3)
	assert.Equal(t, "/etc/frit.yml", got[0])
	assert.Equal(t, filepath.Join("/work", ".frit.yml"), got[1])
	assert.Equal(t, "/home/u/.config/frit/config.yml", got[2])
}

func TestPathsPutsRepoLocalAheadOfUserConfig(t *testing.T) {
	got := Paths(env(map[string]string{
		"HOME": "/home/u",
	}), "/work")

	require.Len(t, got, 2)
	assert.Equal(t, filepath.Join("/work", ".frit.yml"), got[0],
		"a checkout pins its own root before user settings apply")
	assert.Equal(t, "/home/u/.config/frit/config.yml", got[1])
}

func TestPathsPrefersXDGOverHome(t *testing.T) {
	got := Paths(env(map[string]string{
		"HOME":            "/home/u",
		"XDG_CONFIG_HOME": "/xdg",
	}), "/work")

	require.Len(t, got, 2)
	assert.Equal(t, "/xdg/frit/config.yml", got[1])
}

func TestPathsSurvivesAStrippedEnvironment(t *testing.T) {
	got := Paths(env(nil), "/work")

	require.Len(t, got, 1, "only the repo-local file is knowable")
	assert.Equal(t, filepath.Join("/work", ".frit.yml"), got[0])
}

func TestUserConfigDirFallsBackToDotConfig(t *testing.T) {
	assert.Equal(t, "/home/u/.config",
		userConfigDir(env(map[string]string{"HOME": "/home/u"})))
	assert.Equal(t, "/xdg",
		userConfigDir(env(map[string]string{"XDG_CONFIG_HOME": "/xdg"})))
	assert.Empty(t, userConfigDir(env(nil)))
}
