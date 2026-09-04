package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatherStatusInJSONShowsPartialWalk is the projection end to end on
// a representative verb: a fleet of two repositories, one of which the
// walk steps over, so a --json consumer reads read < discovered and a
// problem count rather than inferring coverage from the plans it sees.
func TestGatherStatusInJSONShowsPartialWalk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	good := initRepo(t, root, "atlas")
	commitPlan(t, good, 1, "🔲", "Readable", nil, "")
	broken := initRepo(t, root, "busted")
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var doc report.ReadyDoc

	stderr := emit(t, &doc, "ready", "--root", root)

	assert.Empty(t, stderr, "under --json the progress stays off stderr")
	assert.Equal(t, 2, doc.Gather.Discovered, "both repositories were found")
	assert.Equal(t, 1, doc.Gather.Read, "the broken one was stepped over")
	assert.Equal(t, 1, doc.Gather.Problems, "the step-over is counted")
}

// TestGatherStatusInTableShowsPartialWalk is the same coverage in the
// table: the footer names read of discovered and the problem count, from
// the same model the JSON carries, so a person and a consumer cannot see
// different coverage.
func TestGatherStatusInTableShowsPartialWalk(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	good := initRepo(t, root, "atlas")
	commitPlan(t, good, 1, "🔲", "Readable", nil, "")
	broken := initRepo(t, root, "busted")
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))
	var out, errb bytes.Buffer

	code := run([]string{"ready", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "gathered 1/2 repositories",
		"the table footer names read of discovered")
	assert.Contains(t, out.String(), "1 problem",
		"the table footer names the problem count")
}
