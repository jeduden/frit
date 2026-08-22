package gitobj

import (
	"errors"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
)

// scripted answers a Runner from a table keyed on the joined args,
// erroring for anything not listed — which is how git behaves for a
// ref that does not exist.
func scripted(answers map[string]string) gitwt.Runner {
	return func(_ string, args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		if out, ok := answers[key]; ok {
			return []byte(out), nil
		}

		return nil, errors.New("exit status 1")
	}
}

func TestDefaultRefPrefersTheRemoteHead(t *testing.T) {
	run := scripted(map[string]string{
		"symbolic-ref --quiet refs/remotes/origin/HEAD": "refs/remotes/origin/trunk\n",
		"rev-parse --verify --quiet refs/heads/main":    "abc\n",
	})

	assert.Equal(t, "refs/remotes/origin/trunk", DefaultRef("/r", run))
}

// TestDefaultRefPrefersOriginMainOverALaggingLocalMain pins S84/S85: a
// working checkout's local main advances only on an explicit merge or
// pull, so it routinely lags the remote-tracking default. Landed
// evidence must follow origin's view, not the stale local branch.
func TestDefaultRefPrefersOriginMainOverALaggingLocalMain(t *testing.T) {
	run := scripted(map[string]string{
		"rev-parse --verify --quiet refs/remotes/origin/main": "fresh\n",
		"rev-parse --verify --quiet refs/heads/main":          "stale\n",
	})

	assert.Equal(t, "refs/remotes/origin/main", DefaultRef("/r", run))
}

// TestDefaultRefStillHonorsOriginHeadOverOriginMain confirms adding the
// remote-tracking candidates does not disturb the existing top
// preference: an explicit origin/HEAD still wins even when
// origin/main also resolves.
func TestDefaultRefStillHonorsOriginHeadOverOriginMain(t *testing.T) {
	run := scripted(map[string]string{
		"symbolic-ref --quiet refs/remotes/origin/HEAD":       "refs/remotes/origin/trunk\n",
		"rev-parse --verify --quiet refs/remotes/origin/main": "fresh\n",
		"rev-parse --verify --quiet refs/heads/main":          "stale\n",
	})

	assert.Equal(t, "refs/remotes/origin/trunk", DefaultRef("/r", run))
}

func TestDefaultRefFallsBackToMainThenMaster(t *testing.T) {
	main := scripted(map[string]string{
		"rev-parse --verify --quiet refs/heads/main": "abc\n",
	})
	assert.Equal(t, "refs/heads/main", DefaultRef("/r", main))

	master := scripted(map[string]string{
		"rev-parse --verify --quiet refs/heads/master": "abc\n",
	})
	assert.Equal(t, "refs/heads/master", DefaultRef("/r", master))
}

// TestDefaultRefIgnoresAParkedHead pins the case that motivated the
// cascade: the main worktree sits on a feature branch, and HEAD must
// not be mistaken for the branch work lands on.
func TestDefaultRefIgnoresAParkedHead(t *testing.T) {
	run := scripted(map[string]string{
		"rev-parse --verify --quiet refs/heads/main": "abc\n",
		"symbolic-ref --quiet HEAD":                  "refs/heads/ci/runner-speed\n",
	})

	assert.Equal(t, "refs/heads/main", DefaultRef("/r", run))
}

func TestDefaultRefUsesHeadOnlyAsALastResort(t *testing.T) {
	run := scripted(map[string]string{
		"symbolic-ref --quiet HEAD": "refs/heads/trunk\n",
	})

	assert.Equal(t, "refs/heads/trunk", DefaultRef("/r", run))
}

func TestDefaultRefIsEmptyWhenNoBranchIsSpecial(t *testing.T) {
	assert.Empty(t, DefaultRef("/r", scripted(nil)))
}
