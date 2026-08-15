package report

import (
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zero is the HEAD git reports for a worktree with no commit.
var zero = strings.Repeat("0", 40)

func TestReposCarriesEveryWorktreeOfEveryRepository(t *testing.T) {
	doc := Repos("/fleet", []discover.Repo{{
		Name: "atlas",
		Path: "/fleet/atlas",
		Worktrees: []gitwt.Worktree{
			{Path: "/fleet/atlas", Branch: "main", Head: "a1b2c3"},
			{Path: "/fleet/atlas-lane", Branch: "plan/42-x", Head: zero},
		},
	}})

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "repos", doc.Command)
	assert.Equal(t, "/fleet", doc.Root)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Worktrees, 2)
	assert.Equal(t, "atlas-lane", doc.Repos[0].Worktrees[1].Name)
	assert.False(t, doc.Repos[0].Worktrees[1].HasCommit)
	assert.True(t, doc.Repos[0].Worktrees[0].HasCommit)
}

// TestReposEmitsListsNeverNull pins the half of the contract a
// consumer feels first: iterating a key that is null is an error in
// most languages, and an empty list is not.
func TestReposEmitsListsNeverNull(t *testing.T) {
	doc := Repos("/fleet", nil)

	assert.NotNil(t, doc.Repos)
	assert.Empty(t, doc.Repos)

	withNone := Repos("/fleet", []discover.Repo{{Name: "bare"}})
	assert.NotNil(t, withNone.Repos[0].Worktrees)
}

func TestWorktreeCarriesEveryPorcelainFact(t *testing.T) {
	got := worktreeOf(gitwt.Worktree{
		Path:        "/fleet/atlas-lane",
		Head:        "a1b2c3",
		Branch:      "plan/42-x",
		Detached:    true,
		Bare:        true,
		Locked:      true,
		LockReason:  "in use",
		Prunable:    true,
		PruneReason: "gitdir file points to non-existent location",
	})

	assert.Equal(t, Worktree{
		Path:        "/fleet/atlas-lane",
		Name:        "atlas-lane",
		Head:        "a1b2c3",
		Branch:      "plan/42-x",
		Detached:    true,
		Bare:        true,
		Locked:      true,
		LockReason:  "in use",
		Prunable:    true,
		PruneReason: "gitdir file points to non-existent location",
		HasCommit:   true,
	}, got)
}

func TestWorktreesOfAnEmptyListIsNotNull(t *testing.T) {
	assert.NotNil(t, worktreesOf(nil))
}
