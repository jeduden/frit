---
id: 2608202144
title: The hold is the work ref, a self-healing lease
status: "🔳"
summary: >-
  A verified review found the claim not watertight: the hold ref
  embeds a slug derived from local state, holdership is inferred from
  branch topology and status emoji, and a dead agent blocks its plan
  until a human notices. Redesign: the work ref itself is the lease.
  Every push is a CAS renewal, staleness is observed on one clock,
  takeover and scavenge are CAS transitions, and a fenced zombie is
  rejected by the server. The system detects dead agents and heals
  without human intervention. The full design record, with every
  scenario and its mitigation, is docs/research/lease-protocol.md.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: the lease atom on the work ref
    status: "✅"
  - n: 2
    title: observation, staleness, takeover
    status: "✅"
  - n: 3
    title: scavenge with evidence and park
    status: "🔲"
---
# The hold is the work ref, a self-healing lease

## Goal

Detect dead agents and recover their plans automatically. A plan is
held exactly while its work ref on origin advances under its holder's
CAS pushes. A holder that stops advancing is observed stale on a
single clock and taken over atomically. A zombie that resumes is
rejected by the git server itself. No enumerated scenario needs a
human to recover the fleet.

## Context

A ten-finding adversarial review (2026-08-20) of
[internal/claim](../internal/claim/claim.go),
[cmd/frit/claim.go](../cmd/frit/claim.go),
[cmd/frit/start.go](../cmd/frit/start.go) and
[internal/lanes](../internal/lanes/lanes.go) confirmed the CAS push
atom is sound. It traced the failures to structural roots. The hold
ref name embeds a slug derived from possibly-stale local state, so a
rename lets two machines double-claim. Holdership is inferred from
branch ancestry and status emoji read off a never-fetched view. And
a dead agent blocked its plan until a person released it.

The redesign was then hardened by four blind agents. None saw the
others' work: a scenario enumerator, an independent designer, a
safety attacker, a liveness attacker. The full record is
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md)
— 75 scenarios, 12 liveness traps and 8 safety attacks, each with
its mitigation, plus the rejected alternatives. The protocol below
is the contract distilled from it. Citations such as S27, F9 and A2
refer to entries in that record.

### Reuse

- The CAS push machinery and marker reader in
  [internal/claim](../internal/claim/claim.go) — reused; retargeted
  and extended with epoch trailers.
- The injected-clock idiom of
  [internal/presence](../internal/presence/presence.go) and
  `lanes.Stale` — every staleness rule is pure over a passed `now`,
  so tests state exact times.
- `presence.CachePath()` — the precedent for the observer-state file.
- The fake-runner idiom of
  [internal/gitwt](../internal/gitwt/git.go) — every CAS transition
  is table-driven through a scripted runner.
- [repocfg](../internal/repocfg/config.go) — carries the new `lease:`
  parameters beside `holds:`; legacy hold patterns stay readable
  during the transition.
- Rejected, with reasons in the research note: a separate claims
  namespace (A4), wall-clock TTLs, identity-string fencing (A1),
  renewal keyed to lane-directory activity (F10), a daemon.

## The protocol

One ref per plan: `refs/heads/plan/<id>` — id only, no slug (S27,
S50, S51). It carries the claim marker, work commits, beats, and
release or takeover markers in one chain. The lease token is the tip
SHA. Epoch, a fresh nonce and the holder identity ride as trailers:

```text
plan <id>: claim | beat | release | takeover
epoch:   <E>            increases per acquisition, never per renewal
nonce:   <random>       fresh per commit: no two markers share a SHA
holder:  <machine-id>   stable id; hostname recorded as decoration
lane:    <path>         absolute worktree path on the holder
session: <herdr id>     the pane the lease is bound to; "-" if none
base:    <sha>          claim marker only, the freshly fetched base
```

The nonce is required for correctness (A3). SHA-based CAS is only
ABA-proof if no two commits can hash alike. A deterministic marker
could be recreated at an old SHA, and a pending takeover would then
fire against the fresh lease.

Every transition is one server-side CAS (`--force-with-lease` with
an exact expected value). The server is the arbiter; frit never
decides holdership from a local view.

| Transition | Expected old value      | New tip                              |
| ---------- | ----------------------- | ------------------------------------ |
| acquire    | ref absent              | claim marker, epoch 1, on fresh base |
| re-acquire | release marker tip      | claim marker, epoch E+1, child of it |
| renew      | holder's own last tip   | work commit(s) or beat, same epoch   |
| release    | holder's own last tip   | release marker, same epoch           |
| takeover   | the observed stale tip  | takeover marker, epoch E+1, child    |
| complete   | own tip, landed on main | ref deleted                          |
| scavenge   | the observed tip        | ref deleted; unlanded work parked    |

The rules, each argued and attacked in the research note:

- **Append-only until deletion.** A takeover marker is a child of
  the old tip, so the taker inherits every pushed commit and a
  zombie's history becomes a rejected sibling (S16, S30, A5). An
  unwind pushes a release marker, never a delete (S8, S9, S73).
- **Fencing is the CAS; the token is the tip SHA.** The holder is
  whoever minted the current tip; identity strings are reporting,
  never a check (A1). The token persists in the lane's git dir. On a
  failed renewal a verb re-reads and refuses only a foreign tip
  (A7). Raw git and external side effects are outside the fence and
  inside the trust domain (A8).
- **Staleness is observed non-change on one clock.** Persisted
  per-host windows over the tip: samples spanning more than T with
  no gap over S_max, where S_max is well below T — the defaults set
  S_max = T/4 (F1, F2, S23, S33–36).
  Observation piggybacks on every fleet-reading verb, and `pick`
  surfaces matured takeovers.
- **Liveness precedence.** A positively live bound herdr session
  vetoes takeover and renews on the holder's behalf; no answer is no
  veto; a fenced session's next verb offers `yield` (F3, F10, S61).
- **Self-resume by token.** A lane whose persisted token matches
  origin's tip, with no live session on it, resumes with no window
  (F9, F11, S3). A fleet of one recovers as soon as it restarts.
- **Scavenge on fresh evidence.** Tip-coupled ancestry deletes by
  CAS on exactly the observed tip; glyph or plan-gone evidence also
  requires a matured window, so a renewing holder can never be
  scavenged (A2, F6, F7). Unlanded work is parked to a rescue ref
  first.
- **Yield.** A fenced lane parks its divergence to
  `refs/frit/rescue/<id>/<machine-id>`, tears down via herdr, and
  exits clean; `next` and `show` list rescue refs (F4, F5).
- **Parameters travel with the repo.** R, T, S_max and the k·T
  takeover backoff live in `.frit.yml` (F12, F3). T affects cost,
  never correctness: a wrong takeover is CAS-safe, and the rescue
  ref bounds the wasted work.

## Verb behavior, by state

Every verb meets six states: absent, held-live, held-stale,
held-own, released, landed. Each cell has one prescribed answer.
That table lives with plan 2608211326, which wires it through every
verb. The full record is
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md).

## Non-goals

- No daemon. Every protocol action rides a verb some agent runs;
  herdr's veto and renewal are read through its existing socket.
- No server hooks and no forge features assumed. Branch protection
  on `plan/*` would shrink the trust domain and is recommended in
  docs, never required.
- No cross-machine clock use, ever. Timestamps in markers are for
  humans.
- No second parser and no plan edits. frit still never writes a plan
  file; status stays the shipped skills' concern.

## Tasks

1. Phase 1 — the lease-on-work-ref atom: acquire, renew, release,
   fence, on `refs/heads/plan/<id>`.
2. Phase 2 — observation, staleness and takeover; `pick` surfaces
   matured takeovers.
3. Phase 3 — scavenge with evidence and park: landed refs deleted by
   CAS, unlanded work parked to a rescue ref first.
4. The rest of the protocol — yield, veto and self-resume,
   parameters, the verb-state table, docs — is plan 2608211326,
   which depends on this one. The plan file cap is why it is its
   own plan, not looser scoping.

## Phase 1: the lease atom on the work ref

Delivered (✅). Acquire, renew, release and the fence live in
[internal/claim](../internal/claim/lease.go): one CAS per transition
on `refs/heads/plan/<id>`, nonce-unique markers, epoch E+1 on
re-acquire, and a `start` unwind that pushes a release marker and
names the worktree and pane it stood up. Two reader-side moves kept
the no-regression promise: the default hold patterns gained
`plan/{id}`, and a release-marker tip stopped counting as a hold.
The RED cases are the red commit of the pair that landed it.

## Phase 2: observation, staleness, takeover

Delivered (✅). Staleness is a pure rule over (window, now, T,
S_max) in [internal/discovery](../internal/discovery/stale.go), fed
by a per-host store in [internal/observe](../internal/observe)
beside the presence cache; every fleet-reading verb folds in what it
saw. `claim.Takeover` CASes a marker onto exactly the observed stale
tip at epoch E+1; `ready` and `pick` rank matured takeovers with
their observed age, and a renewal that wins the race leaves the
loser re-reading and resetting its window. One narrowing: read verbs
observe the local view of the remote-tracking ref rather than
fetching, so they stay offline; a takeover against a moved tip
simply loses its CAS.

## Phase 3: scavenge with evidence and park

The ref lifecycle closes: landed evidence deletes the work ref, and
no deletion ever loses work. Ancestry evidence is tied to the tip it
deletes; glyph evidence — ✅ on the default branch while a
live-looking ref lingers, the squash-merge case ancestry cannot see —
additionally requires a matured window, so a renewing holder can
never be scavenged (A2, F6, F7). Scavenge rides the next mutating
verb; read verbs stay read-only.

RED, with the fixture-remote idioms the lease tests use:

- Unlanded commits are parked to
  `refs/frit/rescue/<id>/<machine-id>` before any delete; a
  marker-only ref parks nothing and is simply deleted.
- The tip moved since observation: the CAS fails, nothing is deleted
  or parked over.
- A second scavenge is an idempotent no-op; the rescue ref is
  create-only and never clobbered.
- `claim` of a landed ref with an open status refuses with today's
  wording and scavenges the leftover ref.
- `claim` of a plan ✅ on the default branch whose ref lingers
  scavenges only under a matured window; a fresh window leaves the
  ref alone.

GREEN: `Scavenge` in [internal/claim](../internal/claim/lease.go).
It parks by create-only push, then deletes with one CAS on exactly
the observed tip. `claim` wires it into the landed and already-done
refusals in [cmd/frit/claim.go](../cmd/frit/claim.go).

Gate: the five RED cases pass; a rescue ref always precedes a delete
that would lose work; `go test ./...` and the linter stay green.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                                      |
| ------------------ | ------ | --------- | --------------------------------------------------------------------- |
| 1 lease atom       | opus   | sonnet    | one winner per id, rename-proof, release deletes nothing, fence names |
| 2 observe and take | opus   | sonnet    | matured window takes over; moved tip, gap or lost state never fires   |
| 3 scavenge, park   | opus   | sonnet    | rescue precedes delete; CAS exact tip; renewing holder never lost     |

## Acceptance Criteria

- [ ] Two machines racing a claim resolve to one winner even when the
      plan file was renamed between their fetches
- [ ] A dead holder's plan is taken over automatically after T with
      no human action, and the takeover inherits its pushed work
- [ ] A zombie's push after takeover is rejected by the server, and
      its next verb refuses, names the holder, and offers yield
- [ ] A crashed holder on its own host resumes its lease immediately,
      with no staleness wait
- [ ] An origin outage voids observation windows; recovery triggers
      no takeover of any live holder
- [ ] A merged-but-unreleased hold is scavenged on tip-coupled
      evidence, and scavenge never deletes unlanded work
- [ ] A reopened plan's fresh lease survives a stale ✅ observation:
      a renewing holder can never be scavenged
- [ ] No two marker commits ever share a SHA, pinned by a test over
      identical acquisition inputs
- [ ] Every verb-state cell in the behavior table is a passing test
      with a scripted runner and an explicit clock; no test sleeps
- [ ] Every mechanism in the research note's scenario matrix traces
      to at least one test named for it
- [ ] [docs/claiming.md](../docs/claiming.md) describes the shipped
      lease behavior — its manual-delete recovery and slug-branch
      prose are gone, and its "what changes next" note with them
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
