package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
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

// count reports how many recorded calls began with the given herdr
// subcommand words — verb's counting sibling, for asserting a retry
// ran more than once.
func (h *herdrCalls) count(words ...string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
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
			n++
		}
	}

	return n
}

// hasArg reports whether any recorded call carried the given word
// anywhere in its arguments, for asserting on a flag value — a branch
// passed to worktree create — that verb's leading match cannot see.
func (h *herdrCalls) hasArg(word string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.calls {
		for _, w := range c {
			if w == word {
				return true
			}
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
	return heldPlanModeled(t, root, name, id, title, "")
}

// heldPlanModeled is heldPlan carrying a declared model tier, so a test
// can prove the tier a dispatch verb shows comes from the plan.
func heldPlanModeled(
	t *testing.T, root, name string, id int, title, model string,
) string {
	t.Helper()
	repo := initRepo(t, root, name)
	// Commit the plan on the hold branch, not main: a branch level with
	// main reads as already merged, and a merged ref is landed work, not
	// a live claim. The claim marker beneath the plan commit is what
	// makes the branch an actual hold, not merely a name match
	// (2608212203).
	git(t, repo, "checkout", "-q", "-b", planBranch(id, title))
	git(t, repo, "commit", "--allow-empty", "-q", "-m",
		fmt.Sprintf("plan %d: claim", id))
	writeHeldPlan(t, repo, id, title, model)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", fmt.Sprintf("plan %d", id))

	return repo
}

// writeHeldPlan lays down an in-progress plan file, carrying a model
// tier when one is given.
func writeHeldPlan(t *testing.T, repo string, id int, title, model string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %d\ntitle: %s\nstatus: %q\n", id, title, "🔳")
	if model != "" {
		fmt.Fprintf(&b, "model: %s\n", model)
	}
	b.WriteString("---\n# " + title + "\n")

	path := filepath.Join(repo, "plan",
		fmt.Sprintf("%d_%s.md", id, slugify(title)))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o600))
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

// TestLiveLaneForRejectsASameNamedBranchInAnotherRepo is the guard on
// repo-local ids: two repositories can carry the same hold branch name,
// and a live agent on one must never be dispatched onto for a plan in
// the other. The lane matches only in the plan's own repository.
func TestLiveLaneForRejectsASameNamedBranchInAnotherRepo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	// Both repos carry plan 7 with the same title, so both derive the
	// identical hold branch plan/7-shared-work.
	heldPlan(t, root, "atlas", 7, "Shared work")
	repoB := heldPlan(t, root, "borg", 7, "Shared work")
	runner, _ := recordingHerdr(map[string]any{
		"agent": "claude", "agent_status": "working", "cwd": repoB,
		"pane_id": "wB:p1", "terminal_title_stripped": "in borg",
	})
	rt := &runtime{git: gitwt.Exec, herdr: runner}
	branch := planBranch(7, "Shared work")

	_, found, _, err := liveLaneFor(&cli{},
		discovery.Plan{Repo: "atlas", ID: 7, Holds: []string{branch}}, rt)
	require.NoError(t, err)
	assert.False(t, found,
		"a lane in another repo on the same branch is not this plan's")

	laneB, foundB, _, err := liveLaneFor(&cli{},
		discovery.Plan{Repo: "borg", ID: 7, Holds: []string{branch}}, rt)
	require.NoError(t, err)
	require.True(t, foundB, "the lane in the plan's own repo matches")
	assert.Equal(t, "wB:p1", laneB.Pane.PaneID)
}

// TestOpenCarriesAnUnreachableHerdr drives the socket-failure branch:
// open needs presence live, and a herdr it cannot reach travels in the
// document as a problem rather than crashing the command.
func TestOpenCarriesAnUnreachableHerdr(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: connect: no such file")
	})
	var doc report.OpenDoc

	stderr := emit(t, &doc, "open", "7", "--root", root)

	assert.Empty(t, stderr, "under --json nothing goes to stderr")
	assert.False(t, doc.Focused)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "herdr", doc.Problems[0].Repo)
	assert.Empty(t, doc.NextAction,
		"an unread herdr leaves presence unknown, so start is not named")
}

// TestPresenceUnknownCoversBothUnreadPaths pins the decision open makes
// before it names a start rung: presence is unknown only when it went
// truly unread. An unreachable herdr, or a host that answered with
// nothing at all — no live read and no cache — leaves a lane possible.
// A clean read that found no lane does not, and neither does a host
// served from stale cache: it still contributed (old) presence to the
// search, so a laneless result there is a real one. The remote read
// (real ssh, wall clock) cannot be driven end to end, so the disjuncts
// are pinned on the pure function.
func TestPresenceUnknownCoversBothUnreadPaths(t *testing.T) {
	assert.False(t, presenceUnknown(nil, nil),
		"a clean read that found no lane is not unknown presence")
	assert.True(t, presenceUnknown(errors.New("dial: no socket"), nil),
		"an unreachable herdr leaves presence unknown")
	assert.True(t, presenceUnknown(nil, []hostProblem{{name: "host box", noPresence: true}}),
		"a host with no presence at all leaves presence unknown")
	assert.False(t, presenceUnknown(nil, []hostProblem{{name: "host box"}}),
		"a host served from stale cache still read presence, so it is known")
}

// TestOpenReportsNoLiveLane: a plan nobody is working, and nobody
// holds, has no pane to raise. Open says so plainly and focuses
// nothing. The plan is in progress on main with no hold branch at
// all — HoldNone — so this stays the ordinary laneless case; a held
// plan's own kinds are pinned separately.
func TestOpenReportsNoLiveLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔳", "Dispatch me", nil, "")
	runner, rec := recordingHerdr() // no panes
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "focus"), "nothing to focus")
	assert.Contains(t, out.String(), "no live lane")
	assert.Contains(t, out.String(), "start it with frit start 7",
		"a laneless plan whose presence was read names the start rung")
}

// TestOpenNamesAResumeForATokenThisMachineHolds is Phase 1's own case
// (#122): a held lane whose pane was closed, with its token still
// persisted in the checkout the marker names and no agent anywhere on
// it, reads HoldResumable. open names the resume rather than a bare
// start recommendation that would refuse.
func TestOpenNamesAResumeForATokenThisMachineHolds(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldLaneOwnedBy(t, root, hostname(), "")
	withHerdr(t, emptyRosterHerdr())
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "no live lane")
	assert.Contains(t, got, "resume it with frit start 7",
		"a token this machine holds is named as a resume, not a bare start")
	assert.NotContains(t, got, "start it with",
		"the resumable framing replaces the plain start wording")
}

// TestOpenNamesTheWaitForALaneItCannotProve: the checkout the marker
// names has lost its token — a lane that lost its local state, or a
// cloned machine and a reused path that never had it (A1). open cannot
// prove the hold as this machine's own, so it never names frit start,
// which would still refuse until the takeover window matures; it names
// the wait instead.
func TestOpenNamesTheWaitForALaneItCannotProve(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	_, lane, _ := heldLaneOwnedBy(t, root, hostname(), "")
	dropToken(t, lane)
	withHerdr(t, emptyRosterHerdr())
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "start it with frit start 7",
		"a hold this machine cannot prove would still have frit start refuse")
	assert.Contains(t, got, "takeover window")
}

// TestOpenNamesTheLiveAgentAttendingAHeldLane: the hold's own marker
// binds a session that is still live, read off outside the lane's own
// worktree — the same herdr roster read laneUnattended runs, not the
// branch-matching join liveLaneFor uses to focus a pane. open says a
// live agent is on it rather than naming a start that would refuse or
// interrupt.
func TestOpenNamesTheLiveAgentAttendingAHeldLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldLaneOwnedBy(t, root, hostname(), "wOld:p1")
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"cwd":                     t.TempDir(),
		"pane_id":                 "wOld:p1",
		"agent_session":           map[string]any{"value": "wOld:p1"},
		"terminal_title_stripped": "elsewhere",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"open", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "frit start 7")
	assert.Contains(t, got, "live agent")
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

// idleLane fakes a herdr with one idle agent parked in repo, the target
// a nudge is allowed to prompt.
func idleLane(repo string) map[string]any {
	return map[string]any{
		"agent": "claude", "agent_status": "idle", "cwd": repo,
		"pane_id": "wC:p1", "terminal_title_stripped": "idle lane",
	}
}

// TestNudgeIsDryRunByDefault is the Phase 2 gate: without --go, nudge
// prints the composition it would send and sends nothing.
func TestNudgeIsDryRunByDefault(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(idleLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--phase", "2", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"), "dry-run sends nothing")
	assert.Contains(t, out.String(), "/plan-phase 7 2",
		"the composition is printed")
	assert.Contains(t, out.String(), "wC:p1", "the target pane is printed")
}

// TestNudgeGoSendsTheComposedCommand: with --go, the slash command is
// prompted into the idle lane, whole.
func TestNudgeGoSendsTheComposedCommand(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(idleLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--phase", "2", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "prompt", "wC:p1", "/plan-phase 7 2"),
		"the composed command is sent whole to the pane")
	assert.False(t, rec.verb("agent", "read"), "nudge never reads a reply")
	assert.Contains(t, out.String(), "sent")
}

// TestNudgeDispatchesAPhaselessPlan: a plan small enough to land in one
// go carries no phase ledger. nudge dispatches it whole, composing
// /plan-phase <id> with no --phase and no error, instead of demanding
// one.
func TestNudgeDispatchesAPhaselessPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(idleLane(repo))
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"), "dry-run sends nothing")
	assert.Contains(t, out.String(), "/plan-phase 7",
		"the whole-plan prompt carries no phase token")
}

// TestNudgeRefusesAnAllDonePhasedPlan: a phased ledger whose every
// phase is done has genuinely nothing left to send, unlike a plan
// with no ledger at all — it still refuses.
func TestNudgeRefusesAnAllDonePhasedPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 7, "🔳", "Shader unit", "✅", "✅")
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "has no open phase")
}

// TestNudgeRefusesABusyLane: an agent that is working is not
// interrupted, even with --go.
func TestNudgeRefusesABusyLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr(map[string]any{
		"agent": "claude", "agent_status": "working", "cwd": repo,
		"pane_id": "wC:p1", "terminal_title_stripped": "busy",
	})
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--phase", "2", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"),
		"a working lane is refused, not interrupted")
	assert.Contains(t, out.String(), "refus")
}

// TestNudgeRefusesWhenNoLiveLane: nudge is into an existing lane, so a
// plan nobody is working is refused rather than started.
func TestNudgeRefusesWhenNoLiveLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr() // no panes
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--go", "--phase", "2",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.False(t, rec.verb("agent", "prompt"))
	assert.Contains(t, out.String(), "no live lane")
}

// TestNudgeSaysHerdrUnreachable: with the socket down, nudge refuses on
// presence being unknown, not on an absent lane it never looked for.
func TestNudgeSaysHerdrUnreachable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: connect: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"nudge", "7", "--phase", "2", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "herdr unreachable")
	assert.NotContains(t, out.String(), "no live lane")
}

// TestNudgeSaysPresenceUnknownWhenAHostIsUnread: a configured host that
// went unread — here the no-cache-path degraded mode, where the remote
// cannot be read or reconciled at all — is presence unknown, not an
// absent lane. nudge refuses on that, the way open withholds its action,
// rather than claiming nobody works the plan. The unread host still
// travels in the report and nothing is ever sent.
func TestNudgeSaysPresenceUnknownWhenAHostIsUnread(t *testing.T) {
	isolate(t)
	// Force presence.CachePath to fail, so the configured remote cannot be
	// read or reconciled and comes back noPresence, not merely stale.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Dispatch me")
	runner, rec := recordingHerdr() // local socket, no panes
	withHerdr(t, runner)
	var doc report.NudgeDoc

	emit(t, &doc, "nudge", "7", "--phase", "2", "--go",
		"--hosts", "box", "--root", root)

	assert.Contains(t, doc.Refused, "presence unknown")
	assert.NotContains(t, doc.Refused, "no live lane",
		"an unread host is not an absent lane frit looked for")
	assert.False(t, rec.verb("agent", "prompt"),
		"nothing is sent when presence is unknown")
	require.NotEmpty(t, doc.Problems, "the unread host travels in the report")
}

// TestNudgeTierComesFromThePlan: the tier shown is the plan's declared
// model, never a flag — dispatch is typed, not chosen.
func TestNudgeTierComesFromThePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := heldPlanModeled(t, root, "atlas", 7, "Dispatch me", "sonnet")
	runner, _ := recordingHerdr(idleLane(repo))
	withHerdr(t, runner)
	var doc report.NudgeDoc

	emit(t, &doc, "nudge", "7", "--phase", "2", "--root", root)

	assert.Equal(t, "nudge", doc.Command)
	assert.Equal(t, "/plan-phase 7 2", doc.Prompt)
	assert.Equal(t, "2", doc.Phase)
	assert.Equal(t, "sonnet", doc.Tier)
	assert.False(t, doc.Sent, "a dry run under --json sends nothing")
	assert.Equal(t, "wC:p1", doc.Target)
}
