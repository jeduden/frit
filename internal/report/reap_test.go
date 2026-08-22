package report

import (
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReapAddRepoCarriesEveryKind pins that AddRepo keeps the six
// kinds reap reports apart rather than merging them, the same
// reasoning orphans' own AddRepo already documents.
func TestReapAddRepoCarriesEveryKind(t *testing.T) {
	doc := NewReap("/fleet", true)
	doc.AddRepo("atlas",
		[]ReapedLane{{PlanID: 42, Branch: "plan/42-x"}},
		[]RefusedLane{{PlanID: 43, Branch: "plan/43-y", Reason: "not landed"}},
		[]DroppedHold{{PlanID: 7, Branch: "plan/7", Rescue: "refs/frit/rescue/7/box"}},
		[]RefusedHold{{PlanID: 8, Branch: "plan/8-z", Reason: "decorated"}},
		[]PrunedWorktree{{Kind: "empty"}},
		[]RefusedWorktree{{Kind: "prunable", Reason: "dirty"}},
	)

	require.Len(t, doc.Repos, 1)
	repo := doc.Repos[0]
	require.Len(t, repo.Reaped, 1)
	assert.Equal(t, int64(42), repo.Reaped[0].PlanID)
	require.Len(t, repo.Refused, 1)
	assert.Equal(t, "not landed", repo.Refused[0].Reason)
	require.Len(t, repo.Dropped, 1)
	assert.Equal(t, "refs/frit/rescue/7/box", repo.Dropped[0].Rescue)
	require.Len(t, repo.RefusedHolds, 1)
	assert.Equal(t, "plan/8-z", repo.RefusedHolds[0].Branch)
	require.Len(t, repo.Pruned, 1)
	assert.Equal(t, "empty", repo.Pruned[0].Kind)
	require.Len(t, repo.RefusedPruned, 1)
	assert.Equal(t, "dirty", repo.RefusedPruned[0].Reason)
}

// TestReapAddRepoNormalizesNilToEmpty pins the half of the JSON
// contract a consumer feels first: a repository with nothing to
// report in a given kind still carries `[]` for it, never `null`,
// even when the caller passes a literal nil for every kind.
func TestReapAddRepoNormalizesNilToEmpty(t *testing.T) {
	doc := NewReap("/fleet", false)
	doc.AddRepo("clean", nil, nil, nil, nil, nil, nil)

	require.Len(t, doc.Repos, 1)
	repo := doc.Repos[0]
	assert.NotNil(t, repo.Reaped)
	assert.NotNil(t, repo.Refused)
	assert.NotNil(t, repo.Dropped)
	assert.NotNil(t, repo.RefusedHolds)
	assert.NotNil(t, repo.Pruned)
	assert.NotNil(t, repo.RefusedPruned)
	assert.False(t, repo.Any())
}

// TestReapAddProblemRecordsAnUnreadableRepository matches the rest of
// the ladder's reports: a repository reap could not read is carried
// in the document rather than dropped from it.
func TestReapAddProblemRecordsAnUnreadableRepository(t *testing.T) {
	doc := NewReap("/fleet", true)
	doc.AddProblem("broken", errors.New("no such worktree"))

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "broken", doc.Problems[0].Repo)
}

// TestReapEmitsListsNeverNull pins the document-level half of the same
// contract: a fresh report's own top-level lists are never null.
func TestReapEmitsListsNeverNull(t *testing.T) {
	doc := NewReap("/fleet", true)

	assert.NotNil(t, doc.Repos)
	assert.NotNil(t, doc.Problems)
}

func TestReapRepoAnyReportsWhateverWasFound(t *testing.T) {
	assert.False(t, ReapRepo{}.Any())
	assert.True(t, ReapRepo{Reaped: []ReapedLane{{}}}.Any())
	assert.True(t, ReapRepo{Refused: []RefusedLane{{}}}.Any())
	assert.True(t, ReapRepo{Dropped: []DroppedHold{{}}}.Any())
	assert.True(t, ReapRepo{RefusedHolds: []RefusedHold{{}}}.Any())
	assert.True(t, ReapRepo{Pruned: []PrunedWorktree{{}}}.Any())
	assert.True(t, ReapRepo{RefusedPruned: []RefusedWorktree{{}}}.Any())
}

// TestWorktreeOfMatchesTheUnexportedConversion pins WorktreeOf, the
// exported entry point a caller outside this package (cmd/frit/reap.go)
// builds a document field by field through, against the same
// unexported conversion every in-package Add* method already uses —
// so the two can never quietly drift apart.
func TestWorktreeOfMatchesTheUnexportedConversion(t *testing.T) {
	w := gitwt.Worktree{
		Path: "/fleet/atlas-lane", Head: "a1b2c3", Branch: "plan/42-x",
	}

	assert.Equal(t, worktreeOf(w), WorktreeOf(w))
}
