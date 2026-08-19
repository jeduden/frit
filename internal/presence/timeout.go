package presence

import (
	"fmt"
	"time"

	"github.com/jeduden/frit/internal/herdr"
)

// WithTimeout bounds a runner so a merely-slow host is treated the same
// as a dead one. If the wrapped exec has not returned within d, the
// call returns a timeout error rather than hanging, and reconciliation
// then renders that host stale instead of blocking the board on it.
//
// The wrapped call keeps running in its goroutine — the buffered
// channel lets it deliver into nothing and exit — so bounding the wait
// does not leak the goroutine. Killing the remote process itself would
// need a context handed down to the runner, which is a later
// refinement; here the board's promptness is what the bound protects.
func WithTimeout(exec herdr.ExecFunc, d time.Duration) herdr.ExecFunc {
	return func(name string, args ...string) ([]byte, error) {
		type reply struct {
			out []byte
			err error
		}

		done := make(chan reply, 1)
		go func() {
			out, err := exec(name, args...)
			done <- reply{out: out, err: err}
		}()

		select {
		case r := <-done:
			return r.out, r.err
		case <-time.After(d):
			return nil, fmt.Errorf("%s: timed out after %s", name, d)
		}
	}
}
