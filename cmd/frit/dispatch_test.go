package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"sync"
	"testing"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// herdrCalls records every herdr invocation a dispatch verb makes, so a
// test can assert both what was sent and — just as important for the
// one-way door — what never was.
type herdrCalls struct {
	mu    sync.Mutex
	calls [][]string
}

// verb reports whether any recorded call began with the given herdr
// subcommand words, e.g. verb("agent", "prompt").
func (h *herdrCalls) verb(words ...string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.calls {
		if len(c) < len(words) {
			continue
		}
		match := true
		for i, w := range words {
			if c[i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

// recordingHerdr fakes a herdr socket that answers `agent list` with the
// given panes and records every other call, returning success. It is the
// seam for asserting that open focuses and nothing more.
func recordingHerdr(agents ...map[string]any) (herdr.Runner, *herdrCalls) {
	body, err := json.Marshal(map[string]any{
		"result": map[string]any{"agents": agents},
	})
	if err != nil {
		panic(err)
	}
	rec := &herdrCalls{}

	return func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "agent" && args[1] == "list" {
			return body, nil
		}

		return nil, nil
	}, rec
}

// heldPlan builds a repository carrying an in-progress plan and parks it
// on that plan's hold branch, which is what a lane under active work
// looks like: a claim frit can resolve and a pane herdr can be on.
func heldPlan(t *testing.T, root, name string, id int, title string) string {
	t.Helper()
	repo := initRepo(t, root, name)
	// Commit the plan on the hold branch, not main: a branch level with
	// main reads as already merged, and a merged ref is landed work, not
	// a live claim.
	git(t, repo, "checkout", "-q", "-b", planBranch(id, title))
	commitPlan(t, repo, id, "🔳", title, nil, "")

	return repo
}

// planBranch is the hold branch a held plan is worked on, in the
// convention frit's default pattern matches.
func planBranch(id int, title string) string {
	return "plan/" + strconv.Itoa(id) + "-" + slugify(title)
}

func TestOpenFocusesTheLiveLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(map[string]any{
		"agent":                   "claude",
		"agent_status":            "working",
		"cwd":                     repo,
		"pane_id":                 "wC:p1",
		"terminal_title_stripped": "on the lane",
	})
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "focus", "wC:p1"),
		"open raises the pane the lane runs in")
	assert.Contains(t, out.String(), "7")
}

// TestOpenSendsNoTextAndStartsNoAgent is the Phase 1 gate: open is the
// read-only handoff, so it must never prompt or start.
func TestOpenSendsNoTextAndStartsNoAgent(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(map[string]any{
		"agent": "claude", "agent_status": "idle", "cwd": repo,
		"pane_id": "wC:p1", "terminal_title_stripped": "idle lane",
	})
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"), "open sends no text")
	assert.False(t, rec.verb("agent", "start"), "open starts no agent")
	assert.False(t, rec.verb("agent", "read"), "open never reads a reply")
}

// TestOpenReportsNoLiveLane: a plan nobody is working has no pane to
// raise. Open says so plainly and focuses nothing.
func TestOpenReportsNoLiveLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr() // no panes
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "focus"), "nothing to focus")
	assert.Contains(t, out.String(), "no live lane")
}

func TestOpenEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, _ := recordingHerdr(map[string]any{
		"agent": "claude", "agent_status": "working", "cwd": repo,
		"pane_id": "wC:p1", "terminal_title_stripped": "on the lane",
	})
	withHerdr(t, runner)
	var doc report.OpenDoc

	emit(t, &doc, "open", "7", "--root", root)

	assert.Equal(t, "open", doc.Command)
	assert.Equal(t, int64(7), doc.Plan.ID)
	assert.Equal(t, "atlas", doc.Plan.Repo)
	assert.True(t, doc.Focused)
	assert.Equal(t, "wC:p1", doc.Target)
	assert.Equal(t, "claude", doc.Agent)
}
