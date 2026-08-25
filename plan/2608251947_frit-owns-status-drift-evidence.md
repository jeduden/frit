---
id: 2608251947
title: frit owns the status-drift evidence plan-sync hand-runs git for
status: "🔳"
summary: >-
  plan-sync's skill tells the agent to run `git log --all --grep=<id>`
  and read human-readable git output to find plans whose status drifted
  from git. Enumerating the commits that mention a plan id and deciding
  whether its work landed is index work — "frit indexes, displays" —
  and parsing raw git violates the porcelain-only rule frit holds for
  itself. Add a read verb that reports, per not-done plan, the drift
  evidence (did the work ref land, which commits name the id), reusing
  the landed-detection the fleet walk already runs. The flip stays the
  agent's judgment and the plan edit stays the agent's hand, because
  frit never edits a plan.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: A drift verb reports landed and naming commits per plan
    status: "✅"
  - n: 2
    title: Squash-merge and last-phase evidence, pinned in JSON
    status: "🔲"
  - n: 3
    title: plan-sync reads the verb, retires its raw git
    status: "🔲"
---
# frit owns the status-drift evidence plan-sync hand-runs git for

## Goal

frit reports the git evidence that a not-done plan's status has
drifted. The plan-sync agent then reads structured rows from one walk,
instead of hand-running `git log` and parsing its prose. The judgment
— is it really done despite a revert or a deferred tail — stays the
agent's. So does the plan-file edit: frit never edits a plan.

## Context

The split we hold: the binary indexes, displays and enforces; the
skill judges and recovers. plan-sync's
[step 1 and step 2](../internal/skills/assets/plan-sync/SKILL.md) break
it in one direction — the agent does index work. It runs
`git log --all --oneline --grep="<id>"` and classifies the output.
Enumerating commits that mention an id and reading whether the work
landed is exactly "frit indexes, displays"; and CLAUDE.md's
[Shelling Out To Git](../CLAUDE.md) rule forbids frit from parsing
human-readable git — a rule this step pushes onto the agent instead.
The ambiguity judgment (landed-then-reverted, tail deferred, evidence
only on an unmerged branch) is correctly the skill's and stays there.

**Reuse first.** The landed signal already exists.
[index.LandedIDs](../internal/index/index.go) marks the plans whose
default-branch copy reads done or superseded.
[gitobj.MergedRefs](../internal/gitobj/git.go) reports which work refs
merged into the default branch.
[claim's landedByContent/landedTip](../internal/claim/claim.go) cover
the squash-merge case: a branch tip is no ancestor of the default
branch, yet its content landed.
[fleet.Gather](../internal/fleet/gather.go) already resolves
`DefaultRef`, `Refs`, `MergedRefs` and `LandedIDs` per repo, in the one
walk every read verb shares. The drift verb hooks into that result
rather than walking again.

**What is new** is only the commit enumeration: the commits whose
message names a plan id. That is a `git log` with an explicit
`--format` and `-C <dir>` (porcelain, per the git rule), not the
human-readable form the skill uses today.

**Not a mutation, not a verdict.** frit reports the evidence and the
mechanical landed flag; it does not label a plan done and does not edit
the file. The classification ladder stays in plan-sync, now reading
frit's structured rows instead of raw git text.

**The report model.** Like every command, the verb builds one
[report](../internal/report) document rendered as a table or JSON, with
the [JSON contract](../CLAUDE.md)'s rules pinned in
[testdata](../internal/report/testdata). The verb ships with the skill
that fronts it in the same change: plan-sync already owns this shape of
plan health, so the mention folds there, not into a new skill.

**Name.** `drift` reports status drift; it is a read verb, consistent
with frit never mutating a plan. Confirm the name before Phase 1 —
`reconcile`/`sync` imply a mutation frit does not do.

## Tasks

1. Add `frit drift`: for each 🔲/🔳 plan, report whether its work ref
   landed (reuse `LandedIDs`) and the commits whose message names the
   id, as a table and JSON, proven end to end on one repo.
2. (determined after Phase 1)
3. (determined after Phase 1)

## Phase 1: A drift verb reports landed and naming commits per plan

A read verb that, for each not-done plan in a repo, reports the two
first-order signals: did its work ref land, and which commits name its
id. This slice proves the walk hook and the report shape every later
phase copies.

RED, at the [cmd/frit](../cmd/frit) level, extending the real-repo
fixture the discovery/gather tests already use. Build a repo whose
default branch carries a `🔳` plan and a commit whose subject names
that plan's id, with the plan's work ref merged into the default
branch. Assert `frit drift --json` reports one row for the plan: its
id, `landed: true`, and the naming commit's sha and subject. A `🔲`
plan with no naming commit and no landed ref reports `landed: false`
and an empty `commits` list — never null.

GREEN: add a `driftCmd` reading the shared `gatherFleet` result for the
landed set (`LandedIDs`/`MergedRefs` are already on it). For each
not-done plan it runs one `git log --all --format=<explicit>
--grep=<id>` with `-C <repo>`, parsing only that explicit format. Build
a [report](../internal/report) document — every key present, lists `[]`
not null. Render it as a table, or as JSON under `--json`.

Gate: the RED assertions pass; `frit drift` and `--json` agree on the
same document; `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.

## Phase 2: Squash-merge and last-phase evidence, pinned in JSON

Phase 1 lands the shape with the ancestor-merge landed signal. This
phase completes the evidence and pins the JSON.

RED, at the [internal/report](../internal/report) and
[cmd/frit](../cmd/frit) levels:

- A plan squash-merged (its work-ref tip no ancestor of the default
  branch, its content landed) reads `landed: true`, via
  `claim.landedByContent`/`landedTip`, not only the ancestor case.
- For a plan with `phases:`, the row carries whether a commit for the
  last phase is present (the last-phase GREEN signal the ladder uses),
  as a plain mechanical flag, no verdict.
- A golden `--json` file in
  [testdata](../internal/report/testdata) pins the document: keys
  always present, `commits` is `[]` when empty, a repo frit could not
  read is carried.

GREEN: extend the row with the squash-merge landed check and the
last-phase-commit flag. Compose the existing claim helpers. Record the
golden with `go test ./internal/report -update`, then read the diff.

Gate: the RED cases pass; the golden matches; `go test ./...`,
`go vet ./...`, `golangci-lint run` and `mdsmith check .` stay clean.

## Phase 3: plan-sync reads the verb, retires its raw git

The evidence exists as a verb; this phase moves plan-sync onto it and
removes the hand-run git, closing the split violation.

RED, at [internal/skills](../internal/skills): a test asserts the
plan-sync asset mentions `{{frit}} drift`. It also asserts the asset no
longer carries a raw `git log` invocation. This mirrors the existing
"verb is fronted" guards.

GREEN: rewrite plan-sync's Enumerate and Gather-evidence steps to run
`{{frit}} drift`, and classify its rows with the existing ladder. Drop
the `git log --grep` block. Regenerate the dogfood copies
(`frit skills --via "go run ./cmd/frit" --force .`). Add `drift` to the
verb list and the Shipping Skills note in [CLAUDE.md](../CLAUDE.md),
then run `mdsmith fix AGENTS.md`.

Gate: `frit drift` on a fixture yields the evidence rows the rewritten
step describes — the claim checked against the built binary, not only
linted. The new skills test passes; `TestDogfoodCopiesMatchCanonical`
stays green; every skill stays under the 650-token budget;
`go test ./...`, `golangci-lint run` and `mdsmith check .` stay clean.

## Execution

Phase 1 proves the walk hook and the report shape — the load-bearing
slice. Phase 2 completes the evidence and pins the JSON. Phase 3 is the
skill fold over a proven verb.

| Phase                          | Design | Implement | Gate that catches a wrong answer                                    |
| ------------------------------ | ------ | --------- | ------------------------------------------------------------------- |
| 1 drift reports landed+commits | opus   | sonnet    | `drift --json` names the landed plan's id, sha and subject          |
| 2 squash + last-phase, golden  | opus   | sonnet    | a squash-merged plan reads landed; the JSON golden matches          |
| 3 plan-sync reads the verb     | opus   | sonnet    | plan-sync mentions `drift`, carries no raw `git log`, dogfood green |

## Acceptance Criteria

- [ ] `frit drift` reports, per 🔲/🔳 plan, whether its work landed and
      the commits whose message names the id
- [ ] Squash-merged work reads landed, not only ancestor-merged work
- [ ] `--json` obeys the contract (keys present, lists `[]`, unreadable
      repos carried) and is pinned by a golden
- [ ] frit edits no plan and labels none done — it reports evidence,
      the flip stays the agent's
- [ ] plan-sync runs `{{frit}} drift` and contains no raw `git log`;
      dogfood copies match and stay under budget
- [ ] plan-sync's `drift` step is verified against the built binary,
      not only linted
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
