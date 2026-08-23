---
id: 2608212010
title: Scavenge reads squash-merged work as landed, so it parks nothing that landed
status: "✅"
summary: >-
  Under squash-merge a landed plan's commits are never ancestors of
  the default branch, so hasUnlanded calls every landed lane
  "unlanded" and Scavenge parks a rescue ref for work that is already
  on main. Every landing then leaves a stray ref, and the park
  conflict plan 2608211936 mitigates fires on ordinary lifecycles.
  Teach Scavenge content evidence: a tip whose merge into the fresh
  base changes nothing is landed, whatever ancestry says. Then update
  the scenario docs — the protocol note's Scavenge section and matrix
  rows, and claiming.md — so the spec describes the shipped evidence.
model: sonnet
depends-on: [2608211936]
phases:
  - n: 1
    title: content evidence in the park decision
    status: "✅"
  - n: 2
    title: the scenario docs learn content evidence
    status: "✅"
---
# Scavenge reads squash-merged work as landed, so it parks nothing that landed

## Goal

A scavenged lane whose content already reached the default branch
is deleted without parking a rescue ref. The scenario docs describe
that evidence class.

## Context

[`hasUnlanded`](../internal/claim/lease.go) decides whether
`Scavenge` parks before it deletes. Its only instrument is ancestry:
`git log <tip> ^<base>`, any non-marker subject means "work a delete
would destroy". This repository squash-merges (see the plan
2608211326 PRs), so a landed branch's commits are never ancestors of
the base. Every landed multi-commit lane therefore parks a rescue
ref it does not need. That residue is what made the park conflict of
plan 2608211936 an ordinary event rather than a rare one.

Git already has the right question: merge the tip into the fresh
base and see whether anything changes. `git merge-tree --write-tree
<base> <tip>` answers it in one process, no worktree, and prints the
resulting tree OID on its first line — a stable plumbing format, so
the Shelling Out To Git rules hold. If that tree equals the base's
own tree, everything on the tip is already in the base: landed
content, whatever the commit graph says. A conflict (exit 1) or a
differing tree means real divergence, and the park stands.

The evidence is tied to the tip the same way ancestry evidence is
(A2 in [the protocol note](../docs/research/lease-protocol.md)): it
is computed against the observed tip, and the delete CASes on
exactly that tip, so a holder that renews since moves the tip and
the delete fails harmlessly. No staleness rule changes.

### Reuse

- [`hasUnlanded`](../internal/claim/lease.go) is the one seam; both
  its callers sit inside `Scavenge`. No caller changes.
- `freshBase` already refreshes the base against origin before the
  ancestry read; the merge check reuses the same refreshed base.
- The scripted-runner idiom of
  [lease_test.go](../internal/claim/lease_test.go) already builds
  origins with squash-like histories (`TestScavengeParksUnlandedWorkThenDeletes`
  and its fixtures); the new cases extend that file.

## Non-goals

- No change to when a scavenge fires or to its evidence gates. Glyph
  and plan-gone evidence still require a matured window; ancestry
  and content evidence stay tip-coupled. Only the park decision
  inside `Scavenge` learns the new instrument.
- No cleanup of rescue refs already parked for landed work. Listing
  them is plan 2608211936's orphans sweep; deleting them stays a
  human judgment.
- No change to yield. A fenced lane's local divergence is by
  definition not on the base, so yield keeps parking.

## Tasks

1. Phase 1 — `hasUnlanded` gains the merge-no-op check; a
   squash-landed lane is scavenged with no rescue ref.
2. Phase 2 — the protocol note's Scavenge section and matrix rows,
   and claiming.md, describe content evidence.

## Phase 1: content evidence in the park decision

RED, in [lease_test.go](../internal/claim/lease_test.go):

- A lane whose work was squash-merged — the origin's default branch
  carries one commit with the same cumulative diff, no shared
  ancestry — is scavenged with no rescue ref pushed.
- A lane carrying a commit whose content is not on the base still
  parks first. `TestScavengeParksUnlandedWorkThenDeletes` already
  pins this; it must stay green untouched.
- A lane whose content conflicts with the base — both sides edited
  the same lines — reads as unlanded and parks. The conflict exit
  from `merge-tree` is evidence, not a fault.
- A marker-only chain still deletes without parking, unchanged.

GREEN: after the ancestry walk finds non-marker subjects,
`hasUnlanded` runs `git merge-tree --write-tree <base> <tip>` and
compares the printed tree to `<base>^{tree}`. Equal means landed:
return false. A conflict or a differing tree returns true. Only a
failure to run git at all is an error. The check needs git 2.38+;
[ci.yml](../.github/workflows/ci.yml) already prints the git
version under test, so a too-old runner is loud, not silent.

Docs land in the same commits. The "Landed evidence and scavenge"
section of [docs/claiming.md](../docs/claiming.md) gains the
content-evidence sentence: a tip whose merge into the base changes
nothing is landed. Squash-merged lanes then stop leaving rescue
refs.

Gate: the four RED cases pass; `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: the scenario docs learn content evidence

The scenario record is
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md);
research notes carry dated inline notes when a decision moves.

RED: a sweep of the note for claims phase 1 falsified, each finding
becoming an edit. Deviation: the note had already moved. Plan
2608212218's `reap` verb landed a third evidence class, abandonment,
before this phase ran. So the Scavenge section already defined three
classes, not two. Content evidence joins the existing tip-coupled
class instead of opening a fourth: the first bullet is renamed
"Landed evidence" and now covers both ancestry and the merge-no-op
check, under a dated note naming this plan.

- Matrix row S54 ("SCAV accepts landed evidence, not only the
  glyph") names the evidence that closes it; its mechanism cell now
  includes the merge-no-op check.
- The Scavenge section's promise that a ref carrying unlanded work
  is parked stays true; its example of what counts as unlanded is
  reworded so a squash-landed chain is not the example.

GREEN: the edits, plus a re-read of rows S9, S40 and F6 to confirm
their mechanism cells still hold — PARK still exists, it just stops
firing for landed content. The closing "closed by" list of
[docs/claiming.md](../docs/claiming.md) gains this plan.

Gate: `mdsmith check .` clean; no doc claims scavenge parks landed
content, and the matrix names the shipped evidence.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                                     |
| ------------------ | ------ | --------- | -------------------------------------------------------------------- |
| 1 content evidence | opus   | sonnet    | test: squash-landed scavenges with no rescue; divergence still parks |
| 2 scenario docs    | sonnet | sonnet    | no doc claims landed content is parked; S54 names the evidence       |

## Acceptance Criteria

- [x] A squash-merged lane is scavenged with no rescue ref; a lane
      with real divergence still parks first.
- [x] A content conflict reads as unlanded, never as a fault.
- [x] The protocol note's Scavenge section and matrix row S54
      describe content evidence, under a dated inline note.
- [x] [docs/claiming.md](../docs/claiming.md)'s scavenge section
      names the evidence and its closing list names this plan.
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
