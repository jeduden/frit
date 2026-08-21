// Package observe persists what this host has seen of the fleet's
// work refs: one staleness window per held plan, in a per-host state
// file beside the presence cache. Every fleet-reading verb records
// what it saw; the file is the observer's memory between runs. Losing
// it is safe — absent state reads as "first seen now", which only ever
// delays a takeover.
package observe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeduden/frit/internal/discovery"
)

// State maps an observed plan to its staleness window, keyed by Key.
type State map[string]discovery.Window

// Key names one plan's work ref in the state: repo and id, the same
// pair the fleet keys a plan by on this host.
func Key(repo string, planID int64) string {
	return fmt.Sprintf("%s:%d", repo, planID)
}

// Load reads the state from path. A missing or unreadable file is a
// cold start, not a failure: it returns an empty state so observation
// begins fresh rather than crashing on first run.
func Load(path string) State {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}
	}

	return s
}

// Save writes the state to path as JSON, creating its directory.
//
// The write is atomic — a temp file renamed into place — so one frit
// run reading the state while another writes it sees either the old
// file or the new one. A torn read would unmarshal to a cold start,
// which is safe but forgets every window in progress.
func Save(path string, s State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "observations-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())

		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}

	return os.Rename(tmp.Name(), path)
}

// Path is the default location of the observation state, beside the
// presence cache under the user cache directory — per-host state,
// kept where per-host state belongs.
func Path() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "frit", "observations.json"), nil
}
