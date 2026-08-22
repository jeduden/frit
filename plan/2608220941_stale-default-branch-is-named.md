---
id: 2608220941
title: A fetched remote-tracking ref outrunning main is named
status: "✅"
summary: >-
  frit's discovery reads never fetch, so a repo whose local default
  branch has fallen behind its own already-fetched remote-tracking ref
  — `git fetch` ran, `git merge`/`pull` did not — reads pre-merge plan
  status and held state with nothing saying the evidence is stale
  (S80). Gather already reads every ref in one pass; this names the
  gap as a `fleet.Problem` from data already in hand.
model: sonnet
depends-on: []
phases: []
---
# A fetched remote-tracking ref outrunning main is named

## Goal

When a repository's local default branch is a strict ancestor of its
own already-fetched remote-tracking ref, `Gather` records a
`fleet.Problem`. It names the repo and how far behind it is. A stale
`board`/`ready`/`release` read is explained, not silently trusted.

## Context

Reproduced in this session. PR #47 merged plan 2608212203 on GitHub.
`git fetch origin main` in the primary worktree updated
`refs/remotes/origin/main`, but not `refs/heads/main` — ordinary
git, nobody's bug. Every `frit` read kept showing the plan as
🔳/held until an explicit `git merge --ff-only origin/main`. Nothing
in the report said the two refs had diverged, though the data to say
so was already local.

frit deliberately never fetches on its own. Every read here is
local-only. The one narrow exception is
[internal/claim](../internal/claim)'s `freshBase`, which fetches
solely to answer "has this landed" for a single tip already in hand.
It does not refresh general discovery. This plan does not change
that: it adds no fetch, only a comparison of two refs `Gather`
already has. That catches exactly the case that bit this session — a
fetch already ran, the merge did not — not "nobody ever fetched."

**Reuse first.**
[internal/gitobj/git.go](../internal/gitobj/git.go)'s `Refs` already
lists every local branch and remote-tracking ref in one
`for-each-ref` call.
[internal/fleet/gather.go](../internal/fleet/gather.go)'s
`heldBranches` already runs it per repo. `Ref.Branch()`
([internal/gitobj/parse.go](../internal/gitobj/parse.go)) already
strips both `refs/heads/` and `refs/remotes/<remote>/` down to the
bare branch name. That is exactly how a local default ref and its
remote-tracking counterpart are matched up — no new parser.
`DefaultRef` ([internal/gitobj/git.go](../internal/gitobj/git.go))
already resolves the ref this check watches.

The one new subprocess is `merge-base --is-ancestor`, run only when
the two OIDs actually differ.
[internal/claim/lease.go](../internal/claim/lease.go)'s `isAncestor`
already does exactly this, but is unexported and private to that
package for a one-line check. It is not worth promoting for a single
call site, so `internal/fleet` runs its own.

`internal/doctor` was considered and is the wrong home. It is scoped
to plan-file/mdsmith semantics
([internal/doctor/doctor.go](../internal/doctor/doctor.go)), and
never touches git refs. It is also wired to a separate `frit doctor`
verb a person has to remember to run. This problem belongs beside
every other repo-level fault `Gather` already carries.

## Tasks

1. Compare the default ref against its remote-tracking counterpart in
   `Gather` and record a `fleet.Problem` when the former is behind.

## Acceptance Criteria

- [x] A repo whose local default branch is a strict ancestor of its
      own `refs/remotes/<remote>/<branch>` gets a `fleet.Problem`
      naming the repo and how many commits behind
- [x] A repo whose local default branch matches its remote-tracking
      ref, or carries no such ref at all (never fetched, or no
      remote), gets no such problem
- [x] The problem surfaces in both the table and `--json` for
      `board`, `ready`, and every other verb that already prints
      `Gather`'s problems, with no new plumbing beyond
      `fleet.Problem`
- [x] Scenario S80 is recorded in
      [docs/research/lease-protocol.md](../docs/research/lease-protocol.md)
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
