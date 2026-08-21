// Package observe persists what this host has seen of the fleet's
// work refs: one staleness window per held plan, in a per-host state
// file beside the presence cache. Every fleet-reading verb records
// what it saw; the file is the observer's memory between runs. Losing
// it is safe — absent state reads as "first seen now", which only ever
// delays a takeover.
package observe

import (
	"errors"

	"github.com/jeduden/frit/internal/discovery"
)

// State maps an observed plan to its staleness window, keyed by Key.
type State map[string]discovery.Window

// Key names one plan's work ref in the state: repo and id, the same
// pair the fleet keys a plan by on this host.
func Key(repo string, planID int64) string {
	return ""
}

// Load reads the state from path. A missing or unreadable file is a
// cold start, not a failure.
func Load(path string) State {
	return State{}
}

// Save writes the state to path atomically, creating its directory.
func Save(path string, s State) error {
	return errors.New("not implemented")
}

// Path is the default location of the observation state, beside the
// presence cache under the user cache directory.
func Path() (string, error) {
	return "", errors.New("not implemented")
}
