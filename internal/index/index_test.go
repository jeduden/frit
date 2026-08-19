package index

import (
	"testing"

	"github.com/jeduden/frit/internal/plans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plan renders a minimal plan file with the given id and status.
func plan(id int64, status, title string) []byte {
	return []byte("---\nid: " + itoa(id) + "\ntitle: " + title +
		"\nstatus: \"" + status + "\"\n---\n# " + title + "\n")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}

	return string(d)
}

func TestKeyIsHostRepoAndID(t *testing.T) {
	k := Key{Host: "workstation", Repo: "atlas", ID: 2608142306}

	assert.Equal(t, "workstation:atlas:2608142306", k.String())
}

func TestKeySeparatesCollidingIDsAcrossRepos(t *testing.T) {
	a := Key{Host: "h", Repo: "atlas", ID: 100}
	b := Key{Host: "h", Repo: "beta", ID: 100}

	assert.NotEqual(t, a.String(), b.String(),
		"one repo counts from 1, another uses timestamps; both hit 100")
}

func TestBuildGroupsOneplanAcrossManyRefs(t *testing.T) {
	body := plan(2608142306, "🔳", "Fleet index")
	files := []plans.File{
		{Ref: "refs/heads/main", Path: "plan/a.md", OID: "aaa", Content: body},
		{Ref: "refs/remotes/peer/main", Path: "plan/a.md", OID: "aaa", Content: body},
	}

	got, problems := Build("h", "atlas", "", files)

	require.Empty(t, problems)
	require.Len(t, got, 1, "one plan, not one per ref")
	assert.Equal(t, "h:atlas:2608142306", got[0].Key.String())
	assert.Equal(t, 2, got[0].RefCount())
	require.Len(t, got[0].Versions, 1, "same content is one version")
}

func TestBuildKeepsDistinctVersionsApart(t *testing.T) {
	old := plan(100, "🔲", "Small")
	edited := plan(100, "✅", "Small")
	files := []plans.File{
		{Ref: "refs/heads/main", Path: "plan/a.md", OID: "aaa", Content: old},
		{Ref: "refs/heads/x", Path: "plan/a.md", OID: "bbb", Content: edited},
		{Ref: "refs/heads/y", Path: "plan/a.md", OID: "bbb", Content: edited},
	}

	got, problems := Build("h", "beta", "", files)

	require.Empty(t, problems)
	require.Len(t, got, 1)
	require.Len(t, got[0].Versions, 2)
	assert.Equal(t, "✅", got[0].Primary().Plan.Status,
		"the version most refs agree on wins")
}

// TestLandedIDsRequiresTheStatusOnTheDefaultBranch is the guard the
// squash-merge fix must not overreach: a plan flipped ✅ only on its own
// feature branch — the ordinary pre-merge state the plan-phase workflow
// leaves — has not landed, so its claim is still live and must not be
// suppressed. Primary() falls back off the default branch, so done-ness
// alone is not enough; the authoritative version must be the one the
// preferred ref carries.
func TestLandedIDsRequiresTheStatusOnTheDefaultBranch(t *testing.T) {
	preferred := "refs/heads/main"
	files := []plans.File{{
		Ref: "refs/heads/plan/7-x", Path: "plan/7_x.md", OID: "a",
		Content: plan(7, "✅", "X"),
	}}
	entries, _ := Build("h", "r", preferred, files)

	assert.Empty(t, LandedIDs(entries, preferred),
		"a status flipped only on a feature branch is not landed")
}

// TestLandedIDsMarksAPlanDoneOnTheDefaultBranch is the squash-merge
// case the fix must still catch: the ✅ version is carried by the
// preferred ref, so the work landed however it got there, and a claim
// left behind on it is not a live hold.
func TestLandedIDsMarksAPlanDoneOnTheDefaultBranch(t *testing.T) {
	preferred := "refs/heads/main"
	files := []plans.File{{
		Ref: preferred, Path: "plan/7_x.md", OID: "a",
		Content: plan(7, "✅", "X"),
	}}
	entries, _ := Build("h", "r", preferred, files)

	assert.True(t, LandedIDs(entries, preferred)[7],
		"done on the default branch is landed")
}

func TestRankBreaksTiesOnObjectIDForStability(t *testing.T) {
	versions := []Version{
		{OID: "bbb", Refs: []string{"r1"}},
		{OID: "aaa", Refs: []string{"r2"}},
	}

	rank(versions, "")

	assert.Equal(t, "aaa", versions[0].OID,
		"equal reach, so the order must not depend on map iteration")
}

func TestBuildParsesEachDistinctBlobOnce(t *testing.T) {
	body := plan(7, "✅", "Shared")
	files := make([]plans.File, 0, 50)
	for i := range 50 {
		files = append(files, plans.File{
			Ref:     "refs/heads/b" + itoa(int64(i)),
			Path:    "plan/a.md",
			OID:     "same",
			Content: body,
		})
	}

	got, problems := Build("h", "r", "", files)

	require.Empty(t, problems)
	require.Len(t, got, 1)
	assert.Equal(t, 50, got[0].RefCount())
	assert.Len(t, got[0].Versions, 1)
}

func TestBuildSkipsTheProtoTemplate(t *testing.T) {
	files := []plans.File{{
		Ref:     "refs/heads/main",
		Path:    "plan/proto.md",
		OID:     "aaa",
		Content: []byte("---\nid: 'int & >=1'\n---\n# ?\n"),
	}}

	got, problems := Build("h", "r", "", files)

	assert.Empty(t, got)
	assert.Empty(t, problems, "the template is skipped, not an error")
}

func TestBuildReportsUnparseableFilesWithoutFailing(t *testing.T) {
	files := []plans.File{
		{Ref: "refs/heads/main", Path: "plan/bad.md", OID: "bad",
			Content: []byte("# no front matter\n")},
		{Ref: "refs/heads/main", Path: "plan/good.md", OID: "good",
			Content: plan(1, "✅", "Good")},
	}

	got, problems := Build("h", "r", "", files)

	require.Len(t, problems, 1)
	assert.Contains(t, problems[0].Error(), "plan/bad.md")
	require.Len(t, got, 1, "the good plan still lands")
}

func TestBuildReportsABadBlobOnlyOnce(t *testing.T) {
	bad := []byte("# no front matter\n")
	files := []plans.File{
		{Ref: "refs/heads/a", Path: "plan/bad.md", OID: "bad", Content: bad},
		{Ref: "refs/heads/b", Path: "plan/bad.md", OID: "bad", Content: bad},
		{Ref: "refs/heads/c", Path: "plan/bad.md", OID: "bad", Content: bad},
	}

	_, problems := Build("h", "r", "", files)

	assert.Len(t, problems, 1, "one broken blob, one complaint")
}

func TestBuildSortsEntriesByID(t *testing.T) {
	files := []plans.File{
		{Ref: "r", Path: "plan/b.md", OID: "b", Content: plan(20, "🔲", "B")},
		{Ref: "r", Path: "plan/a.md", OID: "a", Content: plan(10, "🔲", "A")},
	}

	got, _ := Build("h", "r", "", files)

	require.Len(t, got, 2)
	assert.Equal(t, int64(10), got[0].Key.ID)
	assert.Equal(t, int64(20), got[1].Key.ID)
}

func TestBuildOnNoFiles(t *testing.T) {
	got, problems := Build("h", "r", "", nil)

	assert.Empty(t, got)
	assert.Empty(t, problems)
}

func TestPrimaryPrefersTheDefaultBranchOverTheMajority(t *testing.T) {
	// The shape this actually has in the wild: one plan finished on
	// main, and a crowd of old lanes branched before it finished.
	done := plan(42, "✅", "Landed")
	stale := plan(42, "🔲", "Landed")
	files := make([]plans.File, 0, 21)
	files = append(files, plans.File{
		Ref: "refs/heads/main", Path: "plan/a.md",
		OID: "new", Content: done,
	})
	for i := range 20 {
		files = append(files, plans.File{
			Ref:     "refs/heads/old" + itoa(int64(i)),
			Path:    "plan/a.md",
			OID:     "old",
			Content: stale,
		})
	}

	got, _ := Build("h", "atlas", "refs/heads/main", files)

	require.Len(t, got, 1)
	assert.Equal(t, "✅", got[0].Primary().Plan.Status,
		"20 stale lanes do not outvote the branch work lands on")
}

func TestPrimaryFallsBackToTheMajorityWithNoDefaultBranch(t *testing.T) {
	files := []plans.File{
		{Ref: "refs/heads/a", Path: "p.md", OID: "x", Content: plan(1, "✅", "T")},
		{Ref: "refs/heads/b", Path: "p.md", OID: "y", Content: plan(1, "🔲", "T")},
		{Ref: "refs/heads/c", Path: "p.md", OID: "y", Content: plan(1, "🔲", "T")},
	}

	got, _ := Build("h", "r", "", files)

	require.Len(t, got, 1)
	assert.Equal(t, "🔲", got[0].Primary().Plan.Status)
}

func TestOnRefIgnoresAnEmptyPreference(t *testing.T) {
	v := Version{Refs: []string{"refs/heads/main"}}

	assert.True(t, onRef(v, "refs/heads/main"))
	assert.False(t, onRef(v, "refs/heads/other"))
	assert.False(t, onRef(v, ""))
}
