package reap

import (
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wt(path, branch string) gitwt.Worktree {
	return gitwt.Worktree{Path: path, Branch: branch}
}

// TestDecideClearsAWorktreeFritReadsAsLanded pins the gate's open side:
// a branch the caller's own evidence confirms landed is cleared to
// reap, never refused on a guess.
func TestDecideClearsAWorktreeFritReadsAsLanded(t *testing.T) {
	got := Decide(42, []gitwt.Worktree{wt("/w/atlas-landed", "plan/42-fleet")},
		func(string) bool { return true })

	require.Len(t, got, 1)
	assert.Equal(t, int64(42), got[0].PlanID)
	assert.Equal(t, "plan/42-fleet", got[0].Branch)
	assert.Empty(t, got[0].Refused)
}

// TestDecideRefusesAWorktreeFritDoesNotReadAsLanded pins the gate's
// closed side: a branch the caller's evidence does not confirm landed
// is refused rather than cleared, even when the caller already called
// the lane stranded.
func TestDecideRefusesAWorktreeFritDoesNotReadAsLanded(t *testing.T) {
	got := Decide(42, []gitwt.Worktree{wt("/w/atlas-x", "plan/42-fleet")},
		func(string) bool { return false })

	require.Len(t, got, 1)
	assert.NotEmpty(t, got[0].Refused)
}

// TestDecideAsksLandedPerWorktreesOwnBranch: a lane can carry more than
// one worktree on more than one branch name — a legacy decorated one
// alongside the id-only shape — and each is judged on its own branch,
// not the lane's as a whole.
func TestDecideAsksLandedPerWorktreesOwnBranch(t *testing.T) {
	seen := map[string]bool{}
	landed := func(branch string) bool {
		seen[branch] = true
		return branch == "plan/42"
	}

	got := Decide(42, []gitwt.Worktree{
		wt("/w/atlas-legacy", "plan/42-fleet"),
		wt("/w/atlas-id-only", "plan/42"),
	}, landed)

	require.Len(t, got, 2)
	assert.NotEmpty(t, got[0].Refused)
	assert.Empty(t, got[1].Refused)
	assert.True(t, seen["plan/42-fleet"])
	assert.True(t, seen["plan/42"])
}
