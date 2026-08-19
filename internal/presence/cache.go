package presence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/jeduden/frit/internal/herdr"
)

// Fresh reports whether host's snapshot is younger than ttl, so a live
// fleet is not re-probed on every invocation. It is keyed on the host:
// one machine's age never stales another's, and a host with no snapshot
// is never fresh and is always re-probed.
func (c Cache) Fresh(host herdr.Host, ttl time.Duration, now time.Time) bool {
	snap, ok := c[host]
	if !ok {
		return false
	}

	return now.Sub(snap.At) < ttl
}

// Load reads the presence cache from path. A missing or unreadable file
// is a cold start, not a failure: it returns an empty cache so the
// caller re-probes every host rather than crashing on first run.
func Load(path string) Cache {
	data, err := os.ReadFile(path)
	if err != nil {
		return Cache{}
	}

	var c Cache
	if err := json.Unmarshal(data, &c); err != nil {
		return Cache{}
	}

	return c
}

// Store writes the cache to path as JSON, creating its directory. It is
// the one side effect this package owns; Reconcile hands back the cache
// to persist and Store is where it lands.
func Store(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// CachePath is the default location of the presence cache, under the
// user cache directory and named for frit — a cache, kept where caches
// belong rather than beside the config.
func CachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "frit", "presence.json"), nil
}
