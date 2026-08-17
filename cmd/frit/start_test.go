package main

import (
	"bytes"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartComposesTheEscalation: the dry run prints the whole plan —
// the claim, worktree, agent tier and typed prompt — and spawns nothing.
func TestStartComposesTheEscalation(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "dry run")
	assert.Contains(t, got, "plan/7-shader-unit", "the claim branch")
	assert.Contains(t, got, "/plan-phase 7 3", "the typed prompt")
	assert.Contains(t, got, "--model", "the tier maps to an agent arg")
}

// TestStartFoldsInTheNote: a --note rides the composed prompt beneath the
// slash command, so the subject stays the tool's.
func TestStartFoldsInTheNote(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var doc report.StartDoc

	emit(t, &doc, "start", "7", "--phase", "3",
		"--note", "skip the VRT case", "--root", root)

	assert.Equal(t, "/plan-phase 7 3\n\nskip the VRT case", doc.Prompt)
	assert.False(t, doc.Started, "a dry run spawns nothing")
}

// TestStartRefusesAnUnstartablePlan: start mints a claim, so a plan
// already held or blocked is refused rather than escalated.
func TestStartRefusesAnUnstartablePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already held")
}

// TestStartEmitsJSON decodes the escalation a consumer reads to see
// exactly what start would run.
func TestStartEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var doc report.StartDoc

	emit(t, &doc, "start", "7", "--phase", "3", "--root", root)

	assert.Equal(t, "start", doc.Command)
	assert.Equal(t, int64(7), doc.Plan.ID)
	assert.Equal(t, "3", doc.Phase)
	assert.Equal(t, "claude", doc.Kind)
	assert.Equal(t, "plan/7-shader-unit", doc.Branch)
	assert.Equal(t, "/plan-phase 7 3", doc.Prompt)
	assert.NotEmpty(t, doc.Base)
	assert.False(t, doc.Started)
}
