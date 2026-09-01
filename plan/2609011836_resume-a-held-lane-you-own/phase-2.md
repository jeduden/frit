---
n: 2
title: Reattach stands the agent up in the lane, not the caller's pane
status: "✅"
result: false
---
Make the from-outside resume usable. Phase 1 taught `start` to reach the
right lease from a hold this host owns; its stand-up still drives
`pane current`, which from outside the lane is the *caller's* pane, so
the agent would come up in the wrong directory. Reattach must open the
lane's existing checkout and start the agent in the pane that call hands
back.

**Assumes.** `laneStandUpPane` in
[cmd/frit/start.go](../../cmd/frit/start.go) branches on one `resume`
flag. True reads `herdr.CurrentPane`; false calls
`herdr.WorktreeCreate`. A resume cannot use `worktree create`: the
lane's checkout already sits at that path, so herdr answers "already
used by worktree at <path>". `startResume` asks two proofs, and which
one answered is the inside/outside distinction this phase needs. The
cwd-derived token fires only from inside the lane. `ownHoldResumeTip` is
the one a closed pane leaves you outside to read. herdr carries a third
worktree verb frit does not wrap yet. `worktree open` takes `--cwd`,
`--path` and `--label`, is idempotent on an open checkout, and answers
with the `result.root_pane.pane_id` envelope `parseWorktreePane` already
reads.

**Value.** The reattach lands where the work is. A lane whose pane was
closed comes back with its agent in its own checkout, holding its own
commits — the whole point of #122 — rather than starting an agent in
whatever directory the caller happened to be standing in. The
inside-the-lane self-resume is untouched.

**RED.** In
[internal/herdr/dispatch_test.go](../../internal/herdr/dispatch_test.go)
and [cmd/frit/start_test.go](../../cmd/frit/start_test.go).

- `TestWorktreeOpenReturnsTheRootPane`: `WorktreeOpen` asks herdr for
  `worktree open --cwd … --path … --label … --no-focus --json` and
  returns the root pane. No `--base`: nothing is being created.
- `TestWorktreeOpenReportsAMissingPane`: a response with no pane is an
  error, never an empty target an agent is started into.
- `TestStartReattachesInTheLanesOwnPane`: the phase 1 fixture, with the
  herdr fake answering `worktree open` with a *different* pane than
  `pane current`. Assert `agent start` names the pane `worktree open`
  returned, that the open call carries the lane the hold records, and
  that `pane current` is never read — the caller's pane is not the
  lane's.
- `TestStartSelfResumeDoesNotReopenItsOwnLane`: the token resume, run
  from inside the lane, still drives `pane current` and calls no
  `worktree open`. The existing self-resume test grows this assertion.
- `TestStartRefusesWhenAReattachCannotOpenTheLane`: `worktree open`
  fails, so the stand-up fails. The lease already renewed stands — a
  resume never unwinds its own hold — and the error names the open.

**GREEN.** In
[internal/herdr/dispatch.go](../../internal/herdr/dispatch.go), add
`WorktreeOpen` beside `WorktreeCreate`. It takes the same
`WorktreeSpec` and ignores `Base`. Give `parseWorktreePane` the verb to
name in its error, so both wrappers share one reader. In
[cmd/frit/start.go](../../cmd/frit/start.go), carry which proof
answered: `startResume` returns a resumption recording the lane, the tip
and whether the hold rather than the token resolved it.
`laneStandUpPane` then has three ways to a pane — `worktree create` for
a fresh acquire or takeover, `pane current` for the self-resume,
`worktree open` at the resumed lane for the reattach.

**Guard the edges.** The reattach opens the lane the *marker* records,
never `sp.Lane`'s naming convention — the same rule the renewal already
follows, since the two diverge whenever the checkout was set up off that
convention. `pane current` stays the self-resume's read: from inside the
lane it is right, and it is what the existing token path is pinned to. A
failed open is a stand-up failure, so `startExecute`'s resume branch
already keeps the lease rather than releasing a hold that legitimately
stands.

**Gate.** With a hold this host owns and no live agent, `frit start
<id> --go` from outside the lane opens the recorded checkout and starts
the agent in that pane, reading no `pane current`; the self-resume from
inside still uses its own pane; a failed open surfaces as an error with
the lease intact. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
