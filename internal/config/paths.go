// Package config locates frit's configuration files.
//
// Discovery is a pure function of the environment and the working
// directory so it can be tested without touching a real home
// directory, and so the precedence order is stated in one place
// rather than emerging from the order of calls at start-up.
package config

import "path/filepath"

// FileName is the repository-local configuration file.
const FileName = ".frit.yml"

// Paths returns candidate configuration files in precedence order,
// most specific first. Callers hand the list straight to the config
// loader, which takes the first file that both exists and defines
// the key being resolved.
//
// The order is deliberate:
//
//  1. $FRIT_CONFIG, when set, is an explicit override and wins.
//  2. .frit.yml beside the work, so a checkout can pin its own root
//     without touching the user's settings.
//  3. The XDG user configuration, which is the normal home for a
//     personal fleet roster.
//
// Paths that do not exist are harmless: the loader skips them.
func Paths(getenv func(string) string, workdir string) []string {
	paths := make([]string, 0, 3)

	if explicit := getenv("FRIT_CONFIG"); explicit != "" {
		paths = append(paths, explicit)
	}

	paths = append(paths, filepath.Join(workdir, FileName))

	if dir := userConfigDir(getenv); dir != "" {
		paths = append(paths, filepath.Join(dir, "frit", "config.yml"))
	}

	return paths
}

// userConfigDir resolves the XDG configuration base directory,
// falling back to ~/.config. It returns "" when neither variable is
// set, which happens in a stripped environment and is not an error.
func userConfigDir(getenv func(string) string) string {
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config")
	}

	return ""
}
