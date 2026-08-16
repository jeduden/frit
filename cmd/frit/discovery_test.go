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
