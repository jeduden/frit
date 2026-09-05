---
id: 2609050854
title: A claim-only lane carries its token, and every refusal names the way out
status: "🔲"
summary: >-
  frit claim mints the lease, then asks herdr to create the lane's
  worktree. The token that proves the lane is its own is written before
  that worktree exists, so the write fails silently and a claim-only
  lane never carries its proof. frit start has the same order but binds
  the session afterwards, and that renewal writes the token. So a lane
  from start can release and resume itself and a lane from claim
  cannot: release answers "held live by another lane", start answers
  "already held, not takeable until the window matures", and the only
  way through is a two-hour wait meant for cloned machines. Write the
  token once the worktree stands; then make release and start name the
  real situation and the next verb, in the next_action field the
  dispatch verbs already carry; then surface a token-less lane in the
  board, orphans, show and next so a person or a skill meets it before
  a refusal does.
model: sonnet
depends-on: []
---
# A claim-only lane carries its token, and every refusal names the way out

## Goal

A lane that `frit claim` stands up carries its lease token, so
`release` and `start` from inside it work as they do for a lane
`start` created. Where a lane still carries no token, every verb that
meets it says so and names the next command, in one field and one
wording, so an agent branches on the field and a person reads the
sentence.

## Context

**The gap, confirmed by hand.** In a scratch fleet with a bare remote,
`frit claim <id>` claimed the plan and herdr stood the worktree up
beside the repository. `frit release` from inside that worktree then
refused with "is held live by another lane (plan/<id>); only its own
lane can release it". No `frit/token-<id>` file existed under the
lane's git directory. `frit start <id>` from the same lane refused
with "already held, not takeable until the window matures".

**Why the token is missing.** `pushClaimMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) calls
`persistToken` with the lane path the marker records. `claim` runs
that push in `mintClaim`. Only afterwards does it call
`standUpClaimWorktree` in
[cmd/frit/claim.go](../../cmd/frit/claim.go), which asks herdr to
create the worktree. At persist time the directory does not exist.
`TokenPath` in
[internal/claim/token.go](../../internal/claim/token.go) cannot find a
git directory, and the write is skipped by design. A token that could
not be written costs the lane only its resume shortcut.

`start` follows the same order, but `bindSession` in
[cmd/frit/start.go](../../cmd/frit/start.go) renews the lease through
`RenewToBind` once the agent is up. That renewal persists the token
into the worktree that now exists.

**Why the check must stay.** The token is the identity. The
[lease protocol](../../docs/research/lease-protocol.md) rejects
recognising a lane by its holder string or path, because a cloned
machine or a reused path shares both with no race (A1, S49). So
`release` is right to refuse a lane without a token. It is wrong only
in what it says: `foreignHoldRefusal` in
[cmd/frit/release.go](../../cmd/frit/release.go) words every unproved
hold as another lane's, while `inOwnLane` in
[cmd/frit/claim.go](../../cmd/frit/claim.go) already tells "not my
lane" from "my lane with no token" (S77).

**Reuse first.** The write exists: `persistToken` takes a lane path
and a tip, and `claim.Resume` and `RenewToBind` both call it after
a CAS. Phase 1 adds no new proof and no new transition; it calls the
write again once the worktree stands, from the one place that knows it
does. The next-step field exists: `NextAction` on the dispatch
documents in
[internal/report/dispatch.go](../../internal/report/dispatch.go)
carries the exact command for `open`, `nudge`, `start` and
`pick`, and `Ask` on
[internal/report/board.go](../../internal/report/board.go) carries the
`frit message` line for a deserted lane. The categories exist:
`OrphansDoc` in
[internal/report/orphans.go](../../internal/report/orphans.go) already
lists a hold with no worktree behind it. Nothing here adds a second
mechanism for any of the three.

**Out of scope.** Loosening the token rule. A lane with no token still
waits the window or is taken over from another checkout, whatever its
holder string says. Legacy lanes stood up before this plan gain their
token on their next successful renewal, as today; this plan only
guarantees they are told so.

## Tasks

1. Phase 1 (proving slice): `claim` persists the token once herdr
   reports the worktree created. A claim-only lane then releases from
   inside itself and resumes through `start` without waiting the
   window. Pinned by a scenario row and a cmd-level test.
2. Phase 2: `release` and `start`, refused from inside a lane that
   carries no token, say so and carry the next command in
   `next_action`. The wording names the situation, the window, and
   the takeover from another checkout.
3. Phase 3: a held lane on this host whose checkout carries no token is
   surfaced with the same field and wording by `board`, `orphans`,
   and, from inside the lane, `show` and `next`. `plan-drive` and
   `plan-tidy` branch on the field; `plan-phase` stops on it.

## Execution

| Phase | Title                                                    | Tier   | Gate                                                                                                                                        |
| ----- | -------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A claim-only lane persists its token once it stands      | sonnet | claim, then release from the lane, succeeds against the built frit; the token file exists; the new scenario runs; `go test ./...` green     |
| 2     | Refusals from a token-less lane name the way out         | sonnet | release and start from a lane with its token removed print the new wording and a non-empty `next_action`; goldens re-recorded; tests green  |
| 3     | The board, orphans, show and next surface the same state | opus   | each verb's table and JSON carry the field for a token-less lane and nothing for a healthy one; the built frit confirms every skill's claim |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                             |
| --- | ------ | ----------------------------------------------------------------- |
| 1   | 🔲     | [A claim-only lane persists its token once it stands](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] A lane `frit claim` stands up carries `frit/token-<id>` in its
      git directory, holding the minted tip
- [ ] `frit release` from inside a claim-only lane ends the lease, and
      `frit start` from inside it resumes without waiting the window
- [ ] A claim whose worktree herdr cannot create still unwinds and
      leaves no token behind
- [ ] `release` and `start`, refused from inside a lane with no
      token, name that situation and carry a non-empty `next_action`
- [ ] The token rule is unchanged: a lane with no token is never
      released or resumed on its holder string or path
- [ ] `board`, `orphans`, `show` and `next` surface a token-less
      held lane on this host with the same field and wording, and show
      nothing extra for a healthy lane
- [ ] The shipped skills branch on the field, and each claim in them is
      confirmed against the built frit
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
