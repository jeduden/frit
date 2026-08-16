package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commitPlan writes a plan file on the current branch and commits it.
// deps is the depends-on list, empty for none; body is appended after
// the front matter so a test can add phase sections.
func commitPlan(
	t *testing.T, repo string, id int, status, title string,
	deps []int, body string,
) {
	t.Helper()
	writePlanFile(t, repo, id, status, title, deps, "", body)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))
}

// writePlanFile lays down one plan file with full front matter, without
// committing it. phases is a raw YAML block for the phases: key, empty
// for none.
func writePlanFile(
	t *testing.T, repo string, id int, status, title string,
	deps []int, phases, body string,
) {
	t.Helper()
	path := filepath.Join(repo, "plan", fmt.Sprintf("%d_%s.md", id,
		slugify(title)))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))

	var fm strings.Builder
	fmt.Fprintf(&fm, "---\nid: %d\ntitle: %s\nstatus: %q\n", id, title,
		status)
	if len(deps) > 0 {
		parts := make([]string, len(deps))
		for i, d := range deps {
			parts[i] = fmt.Sprint(d)
		}
		fmt.Fprintf(&fm, "depends-on: [%s]\n", strings.Join(parts, ", "))
	}
	if phases != "" {
		fm.WriteString(phases)
	}
	fm.WriteString("---\n# " + title + "\n")
	if body != "" {
		fm.WriteString("\n" + body + "\n")
	}

	require.NoError(t, os.WriteFile(path, []byte(fm.String()), 0o600))
}

// slugify makes a filename-safe slug from a title.
func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}

		return '-'
	}, s)

	return strings.Trim(s, "-")
}

// phasesBlock builds a phases: front-matter block, one entry per given
// status, numbered from one.
func phasesBlock(statuses ...string) string {
	var b strings.Builder
	b.WriteString("phases:\n")
	for i, status := range statuses {
		fmt.Fprintf(&b, "  - { n: %d, title: 'Phase %d', status: %q }\n",
			i+1, i+1, status)
	}

	return b.String()
}

// TestReadyListsAPlanWithDepsDoneAndWithholdsAnUnmetOne is the flagship
// end to end: a plan whose every dependency is ✅ is startable, and one
// with an unmet edge is not.
func TestReadyListsAPlanWithDepsDoneAndWithholdsAnUnmetOne(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Foundation", nil, "")
	commitPlan(t, repo, 2, "✅", "Middle", nil, "")
	commitPlan(t, repo, 3, "🔲", "Startable now", []int{1, 2}, "")
	commitPlan(t, repo, 4, "🔳", "Underway", nil, "")
	commitPlan(t, repo, 5, "🔲", "Still blocked", []int{4}, "")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Startable now")
	assert.NotContains(t, got, "Still blocked",
		"a plan waiting on an unfinished plan is withheld")
}

// TestReadyWithholdsAHeldPlan: a not-started plan a lane already claims
// is not offered, even with no dependencies.
func TestReadyWithholdsAHeldPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 2608161809, "🔲", "Claimed already", nil, "")
	claimBranch(t, repo, "plan/2608161809-discovery")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable")
}

func TestReadyIsQuietWhenNothingIsStartable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "All done here", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable")
}

func TestReadyEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Foundation", nil, "")
	commitPlan(t, repo, 3, "🔲", "Startable now", []int{1}, "")
	var doc report.ReadyDoc

	emit(t, &doc, "ready", "--root", root)

	assert.Equal(t, "ready", doc.Command)
	assert.NotEmpty(t, doc.Host)
	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(3), doc.Plans[0].ID)
	assert.Equal(t, "atlas", doc.Plans[0].Repo)
	assert.Equal(t, []int64{1}, doc.Plans[0].DependsOn)
}

// commitPhasedPlan writes and commits a plan carrying a phase ledger.
func commitPhasedPlan(
	t *testing.T, repo string, id int, status, title string,
	statuses ...string,
) {
	t.Helper()
	writePlanFile(t, repo, id, status, title, nil,
		phasesBlock(statuses...), "")
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))
}

// TestNextReportsTheFirstOpenPhase is the rule end to end: next skips
// the done phases and stops at the first still open.
func TestNextReportsTheFirstOpenPhase(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔳", "🔲")
	var out, errb bytes.Buffer

	code := run([]string{"next", "100", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "phase 2")
	assert.NotContains(t, got, "phase 1", "a done phase is skipped")
}

// TestNextInfersThePlanFromTheCwd is the third selector form: standing
// in a worktree on a plan branch, next means the plan that worktree is
// working, with no id typed.
func TestNextInfersThePlanFromTheCwd(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "checkout", "-q", "-b", "plan/100-layered")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔲")
	t.Chdir(repo)
	var out, errb bytes.Buffer

	code := run([]string{"next", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "phase 2")
}

func TestNextEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 100, "🔳", "Layered work", "✅", "🔳")
	var doc report.NextDoc

	emit(t, &doc, "next", "layered", "--root", root)

	assert.Equal(t, "next", doc.Command)
	assert.True(t, doc.HasPhase)
	assert.Equal(t, 2, doc.Phase.N)
	assert.Equal(t, int64(100), doc.Plan.ID)
}

// TestShowWalksTheUpstreamDAG: show --deps prints the whole upstream
// chain of a plan.
func TestShowWalksTheUpstreamDAG(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 2, "🔳", "Middle layer", []int{1}, "")
	commitPlan(t, repo, 3, "🔲", "Top", []int{2}, "")
	var out, errb bytes.Buffer

	code := run([]string{"show", "3", "--deps", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "Top")
	assert.Contains(t, got, "Middle layer")
	assert.Contains(t, got, "Bedrock")
}

func TestShowEmitsTheDependencyTree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "Bedrock", nil, "")
	commitPlan(t, repo, 3, "🔲", "Top", []int{1}, "")
	var doc report.ShowDoc

	emit(t, &doc, "show", "3", "--deps", "--root", root)

	assert.Equal(t, "show", doc.Command)
	assert.Equal(t, int64(3), doc.Tree.ID)
	require.Len(t, doc.Tree.Deps, 1)
	assert.Equal(t, int64(1), doc.Tree.Deps[0].ID)
	assert.True(t, doc.Tree.Deps[0].Found)
}

// TestSelectorAmbiguityPrintsCandidatesAndExitsNonZero is the acceptance
// criterion for the selector: a fragment matching two plans is refused
// with its candidates, not resolved by a guess.
func TestSelectorAmbiguityPrintsCandidatesAndExitsNonZero(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Shared word alpha", nil, "")
	commitPlan(t, repo, 2, "🔲", "Shared word beta", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"next", "Shared word", "--root", root}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "matches 2 plans")
	assert.Contains(t, errb.String(), "alpha")
	assert.Contains(t, errb.String(), "beta")
}

func TestSelectorNotFoundExitsNonZero(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Something", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"show", "raymarch", "--root", root}, &out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "no plan matches")
}

// TestFindMatchesAcrossBranches is the payoff of reading every ref: a
// plan that lives only on a side branch is still found by its topic.
func TestFindMatchesAcrossBranches(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "checkout", "-q", "-b", "plan/500-raymarch")
	commitPlan(t, repo, 500, "🔲", "Raymarch the gas giants", nil, "")
	git(t, repo, "checkout", "-q", "main")
	var out, errb bytes.Buffer

	code := run([]string{"find", "raymarch", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "Raymarch the gas giants")
}

func TestFindMatchesASummary(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	path := filepath.Join(repo, "plan", "1_a.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(
		"---\nid: 1\ntitle: A plain title\nstatus: \"🔲\"\n"+
			"summary: walk the signed distance field\n---\n# A\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan 1")
	var out, errb bytes.Buffer

	code := run([]string{"find", "signed distance", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "A plain title")
}

func TestFindIsQuietOnNoMatch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Something", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"find", "raymarch", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no plan matches")
}

func TestFindEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Raymarch the gas giants", nil, "")
	var doc report.FindDoc

	emit(t, &doc, "find", "raymarch", "--root", root)

	assert.Equal(t, "find", doc.Command)
	assert.Equal(t, "raymarch", doc.Query)
	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(7), doc.Plans[0].ID)
}

// TestPickRanksAndTrims: the most-unblocking plan comes first, and -n
// trims the list.
func TestPickRanksAndTrims(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔲", "Frees nothing", nil, "")
	commitPlan(t, repo, 2, "🔲", "Frees two downstream", nil, "")
	commitPlan(t, repo, 3, "🔳", "Waits on two", []int{2}, "")
	commitPlan(t, repo, 4, "🔳", "Also waits on two", []int{2}, "")
	var doc report.PickDoc

	emit(t, &doc, "pick", "-n", "1", "--root", root)

	require.Len(t, doc.Plans, 1)
	assert.Equal(t, int64(2), doc.Plans[0].ID,
		"the plan freeing the most downstream work ranks first")
}
