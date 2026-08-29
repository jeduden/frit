package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/jeduden/frit/internal/herdr"
)

// WithTimeout bounds a context-aware exec so a merely-slow host is
// treated the same as a dead one. If the wrapped exec has not
// returned within d, the call returns a timeout error rather than
// hanging, and reconciliation then renders that host stale instead of
// blocking the board on it.
//
// It builds a context bounded by d and calls the context-aware base
// through it, so exec.CommandContext kills the remote probe — a slow
// `ssh <host> herdr` — the moment the bound fires rather than leaving
// it to run on after the caller gives up.
func WithTimeout(exec herdr.ContextExecFunc, d time.Duration) herdr.ExecFunc {
	return func(name string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()

		out, err := exec(ctx, name, args...)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("%s: timed out after %s", name, d)
		}
		return out, err
	}
}
