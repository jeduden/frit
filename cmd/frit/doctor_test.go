package main

import (
	"bytes"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot is frit's own repository root, resolved from this file's
// own path rather than the working directory: isolate(t) changes cwd
// to a temp directory, so a relative "../../plan/proto.md" only works
// before that runs.
var repoRoot = func() string {
	_, file, _, _ := goruntime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..")
}()

// writeDoctorSchema copies frit's own plan/proto.md and .mdsmith.yml
// into repo, so doctor has a real schema to validate against — the
// same reason internal/doctor's own tests copy rather than hand-roll
// a trimmed-down config: required-structure's kind-assignment
// validation is easy to get subtly wrong by hand.
func writeDoctorSchema(t *testing.T, repo string) {
	t.Helper()
	proto, err := os.ReadFile(filepath.Join(repoRoot, "plan", "proto.md"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "plan"), 0o750))
	require.NoError(t,
		os.WriteFile(filepath.Join(repo, "plan", "proto.md"), proto, 0o600))

	cfg, err := os.ReadFile(filepath.Join(repoRoot, ".mdsmith.yml"))
	require.NoError(t, err)
	require.NoError(t,
		os.WriteFile(filepath.Join(repo, ".mdsmith.yml"), cfg, 0o600))
}

const gappedPlan = `---
id: 100
title: Gapped
status: "🔲"
model: sonnet
---
# Gapped

## Goal

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`

const cleanDoctorPlan = `---
id: 101
title: Clean
status: "🔲"
model: sonnet
---
# Clean

## Goal

Ship it.

## Tasks

1. x

## Acceptance Criteria

- [ ] y
`

// TestDoctorListsAGappedPlanAndOmitsACleanOne is the phase-4 gate
// verbatim: a gapped plan is listed, a clean one is not.
func TestDoctorListsAGappedPlanAndOmitsACleanOne(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	writeDoctorSchema(t, repo)
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "plan", "100_gapped.md"), []byte(gappedPlan), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "plan", "101_clean.md"), []byte(cleanDoctorPlan),
		0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plans")
	var doc report.DoctorDoc

	emit(t, &doc, "doctor", "--root", root)

	require.Len(t, doc.Findings, 1)
	assert.Equal(t, int64(100), doc.Findings[0].ID)
	assert.Equal(t, "goal", doc.Findings[0].Check)

	var out, errb bytes.Buffer
	code := run([]string{"doctor", "--root", root}, &out, &errb)
	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "100")
	assert.NotContains(t, out.String(), "101",
		"the clean plan is omitted from the table")
}

// TestDoctorSkipsARepositoryWithNoSchema: doctor validates against a
// repository's own plan/proto.md, so a repository with none has
// nothing to check — not a Problem, just no findings.
func TestDoctorSkipsARepositoryWithNoSchema(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "plan"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "plan", "100_gapped.md"), []byte(gappedPlan), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "plan")
	var doc report.DoctorDoc

	emit(t, &doc, "doctor", "--root", root)

	assert.Empty(t, doc.Findings)
	assert.Empty(t, doc.Problems,
		"no schema is an accepted limitation, not a repo doctor failed to read")
}

// TestDoctorHelpListsTheChecksAndTheirProvenance: --help is the
// contract for what doctor promises to catch and where each finding
// comes from.
func TestDoctorHelpListsTheChecksAndTheirProvenance(t *testing.T) {
	isolate(t)
	var out, errb bytes.Buffer

	code := run([]string{"doctor", "--help"}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	for _, want := range []string{
		"goal", "schema", "execution-row", "tier",
		"plan/proto.md", "github.com/jeduden/mdsmith/pkg/mdsmith",
	} {
		assert.Contains(t, got, want)
	}
}
