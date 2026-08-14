package gitwt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// porcelainFixture covers every record shape `git worktree list
// --porcelain` emits: a normal checkout, a worktree whose branch has
// no commit yet, a detached HEAD, a bare repository, and the locked
// and prunable markers with and without a reason.
const porcelainFixture = `worktree /home/u/git/atlas
HEAD 88559bff1c0d4e3a9b8f7654321fedcba9876543
branch refs/heads/ci/runner-speed

worktree /home/u/git/atlas-a-hold-is-a-ref-name
HEAD 0000000000000000000000000000000000000000
branch refs/heads/plan/2608051410-a-hold-is-a-ref-name

worktree /home/u/git/atlas-better-pyramid
HEAD 6cdf59a71234567890abcdef1234567890abcdef
detached

worktree /home/u/git/bare-mirror
bare

worktree /home/u/git/atlas-locked
HEAD aaaabbbbccccddddeeeeffff00001111222233334
branch refs/heads/wip
locked on a removable drive

worktree /home/u/git/atlas-prunable
HEAD bbbbccccddddeeeeffff000011112222333344445
detached
prunable gitdir file points to non-existent location
`

func TestParseWorktreeListReadsEveryRecordShape(t *testing.T) {
	got := ParseWorktreeList([]byte(porcelainFixture))
	require.Len(t, got, 6)

	assert.Equal(t, "/home/u/git/atlas", got[0].Path)
	assert.Equal(t, "ci/runner-speed", got[0].Branch)
	assert.False(t, got[0].Detached)
	assert.True(t, got[0].HasCommit())

	assert.Equal(t, "plan/2608051410-a-hold-is-a-ref-name", got[1].Branch)
	assert.False(t, got[1].HasCommit(), "all-zero HEAD means nothing landed")

	assert.True(t, got[2].Detached)
	assert.Empty(t, got[2].Branch)
	assert.True(t, got[2].HasCommit())

	assert.True(t, got[3].Bare)
	assert.Empty(t, got[3].Head)
	assert.False(t, got[3].HasCommit())

	assert.True(t, got[4].Locked)
	assert.Equal(t, "on a removable drive", got[4].LockReason)

	assert.True(t, got[5].Prunable)
	assert.Equal(t,
		"gitdir file points to non-existent location",
		got[5].PruneReason)
}

func TestParseWorktreeListStripsRefsHeadsPrefixOnly(t *testing.T) {
	// A branch named so that a naive TrimPrefix of "refs/heads/"
	// applied twice, or a TrimLeft, would corrupt it.
	in := "worktree /w\nHEAD " + strings.Repeat("a", 40) +
		"\nbranch refs/heads/refs/heads/odd\n"
	got := ParseWorktreeList([]byte(in))
	require.Len(t, got, 1)
	assert.Equal(t, "refs/heads/odd", got[0].Branch)
}

func TestParseWorktreeListIgnoresBlankAndUnknownLines(t *testing.T) {
	in := "\n\nworktree /w\nHEAD " + strings.Repeat("b", 40) +
		"\nsomething-new-in-git 1\n\n\n"
	got := ParseWorktreeList([]byte(in))
	require.Len(t, got, 1)
	assert.Equal(t, "/w", got[0].Path)
}

func TestParseWorktreeListEmptyInputYieldsNoRecords(t *testing.T) {
	assert.Empty(t, ParseWorktreeList(nil))
	assert.Empty(t, ParseWorktreeList([]byte("\n\n")))
}

func TestLockedWithoutReasonStillMarksLocked(t *testing.T) {
	in := "worktree /w\nHEAD " + strings.Repeat("c", 40) +
		"\ndetached\nlocked\n"
	got := ParseWorktreeList([]byte(in))
	require.Len(t, got, 1)
	assert.True(t, got[0].Locked)
	assert.Empty(t, got[0].LockReason)
}

func TestNameIsTheWorktreeDirectoryBasename(t *testing.T) {
	got := ParseWorktreeList([]byte(porcelainFixture))
	require.Len(t, got, 6)
	assert.Equal(t, "atlas", got[0].Name())
	assert.Equal(t, "atlas-better-pyramid", got[2].Name())
}
