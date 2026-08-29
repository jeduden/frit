---
id: 2608291854
title: Harden the headroom check and the signal it ships
status: "🔳"
summary: >-
  The headroom signal shipped in PR #103 (plan 2608290818) has cleanup
  a code review surfaced. `doctor` runs the check on every plan, so a
  done ✅ plan near the cap is a permanent finding an agent can never
  fix. `NoHeadroom` is derivable from `HeadroomShort`, duplicated across
  four layers. The oracle runs on all fifteen fleet verbs but only
  `ready`/`pick` render it, and mdsmith v0.54.0 now exposes the cap
  `internal/headroom` binary-searches for. `doctor` and `headroom` open
  two mdsmith sessions with different missing-config policy. The signal
  ships with no skill text. `pad` and `fits` have no test.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: doctor skips a done plan's headroom check
    status: "✅"
  - n: 2
    title: the headroom signal is one number, not a number and a bool
    status: "✅"
  - n: 3
    title: doctor and headroom share one session opener
    status: "✅"
  - n: 4
    title: the oracle runs only where its answer is shown
    status: "✅"
  - n: 5
    title: the headroom signal ships its skill text
    status: "🔲"
  - n: 6
    title: pad and fits carry their own tests
    status: "🔲"
---
# Harden the headroom check and the signal it ships

## Goal

The headroom signal shipped in PR #103 is made as honest and cheap as
the rest of frit. It stops flagging plans nobody can fix. It carries one
number, not a redundant number-and-bool. It opens one mdsmith session,
runs its oracle only where the answer is shown, ships the skill text an
agent reads, and covers `pad` and `fits` with tests.

## Context

A `/code-review` of PR #103 ([plan
2608290818](2608290818_headroom-for-another-phase.md)) surfaced these,
each verified against the tree and the mdsmith v0.54.0 API. They are
cleanup of a landed feature, not a bug that breaks it; a separate plan
so #104's claim fixes are not held behind them.

- **doctor flags done plans.** `scanFile` in
  [doctor.go](../internal/doctor/doctor.go) runs `checkHeadroom` for
  every plan with a reserve set, with no status filter. A ✅ plan near
  the cap will never gain another phase, yet `doctor` names it, and
  `plan-new` step 7 tells an agent to fix what `doctor` names. frit's
  own repo already shows two such rows (`2608271957`, `2608280653`).
- **`NoHeadroom` is derivable.** `headroomFor` in
  [gather.go](../internal/fleet/gather.go) only ever sets
  `headroomInfo{No: true, Short: reserve - room}` when `room < reserve`,
  so `No == (Short > 0)` always. The pair is copied through
  `headroomInfo`, [discovery.Plan](../internal/discovery/discovery.go)
  and [report.PlanCard](../internal/report/discovery.go) plus the JSON.
- **The oracle runs everywhere.** `headroomFor` runs inside every
  `gatherFleet`, which fifteen verbs call, but only `ready`/`pick`
  render the result. Each `Room` binary-searches with several full
  `Session.Check` passes per plan. And mdsmith v0.54.0 now exposes the
  effective cap directly — `Session.Kinds(uri).Rules["max-file-length"].Final`
  carries `{"max": N}` — so the premise in
  [headroom.go](../internal/headroom/headroom.go)'s package doc, that
  the cap is not reachable, no longer holds.
- **Two sessions.** `headroom.Session` falls back to `ConfigYAML("")`
  when a repo has no `.mdsmith.yml`; `doctor.Scan` builds its own
  session with a bare `ConfigPath` and requires the file. They disagree
  on the no-config case and are opened twice.
- **No skill text.** `plan-pick` never names the new `pick`/`ready`
  column or the `no_headroom`/`headroom_short` keys, and `plan-new`
  step 7 still lists doctor's checks without `headroom`. CLAUDE.md's
  Shipping Skills rule requires the skill in the same change as the verb.
- **Untested helpers.** `pad` and `fits` in
  [headroom.go](../internal/headroom/headroom.go) have no dedicated
  test, and `pad`'s trailing-newline branch is a defensive branch no
  test drives — against CLAUDE.md's Defensive Code and dedicated-test
  rules.

## Tasks

1. Skip the headroom check for a plan whose status is ✅ or ⛔.
2. Collapse `NoHeadroom` into `HeadroomShort` across every layer.
3. Share one mdsmith session opener between `doctor` and `headroom`.
4. Run the oracle only for the verbs that show it, reading the exposed
   cap where that removes a pass.
5. Ship the `plan-pick` and `plan-new` skill text for the signal.
6. Add dedicated tests for `pad` and `fits`, driving the defensive
   branch red first.

## Phase 1: doctor skips a done plan's headroom check

`scanFile` in [doctor.go](../internal/doctor/doctor.go) runs
`checkHeadroom` only for a plan still open to another phase — status 🔲
or 🔳. A ✅ or ⛔ plan is skipped, since it will never grow a phase and
the finding is unactionable. The same gate is applied wherever the fleet
computes the signal, so `pick`/`ready` never surface a done plan's
shortfall either.

RED, a `doctor` test in
[doctor_test.go](../internal/doctor/doctor_test.go):

- A ✅ plan padded past its reserve yields no `headroom` finding.
- A 🔲 plan past its reserve still yields the finding, unchanged.

GREEN: read the parsed status in `scanFile` — planmeta already carries
it — and skip `checkHeadroom` for a done status. Mirror the gate in
`headroomFor` in [gather.go](../internal/fleet/gather.go) against the
entry's status.

Gate: the tests pass; then build frit and run `frit doctor` in this
repo — the two ✅ rows (`2608271957`, `2608280653`) are gone, and a live
🔲 plan past its reserve is still named. `go test ./...` and
`mdsmith check .` clean.

## Phase 2: the headroom signal is one number, not a number and a bool

`HeadroomShort int` alone carries the signal — `0` means the plan has
room, a positive value is the shortfall — and `NoHeadroom` is dropped.
Every reader that tested `NoHeadroom` tests `HeadroomShort > 0` instead:
the `pick`/`ready` label, the table column, the JSON. `headroomInfo`
collapses to `map[int64]int`.

RED: the golden files and the label test assert on `headroom_short`
alone. A `pick`/`ready` test checks a plan with room carries `0` and a
capped plan carries its shortfall. `no_headroom` is gone from every
golden.

GREEN: drop `NoHeadroom` from `headroomInfo`,
[discovery.Plan](../internal/discovery/discovery.go) and
[report.PlanCard](../internal/report/discovery.go). Change the one
`headroomLabel` site to test `HeadroomShort > 0`. Re-record the goldens.

Gate: the label and card tests pass; goldens re-recorded with `go test
./internal/report -update` and the diff read — `no_headroom` removed
from each, every other key present; `go test ./...` and `mdsmith check
.` clean.

## Phase 3: doctor and headroom share one session opener

`doctor.Scan` opens its mdsmith session through `headroom.Session`. Then
there is one config-lookup policy: the repo's `.mdsmith.yml` when
present, mdsmith's built-in defaults when absent. The two sites that
disagree on the no-config case become one. `doctor`'s own schema
requirement — `ErrNoSchema` on a missing `plan/proto.md` — is unchanged;
only the session open moves.

RED, a `doctor` test: a repo with `plan/proto.md` but no `.mdsmith.yml`
scans on defaults rather than failing to open the session. That is the
behaviour `headroom.Session` already has and `doctor` did not.

GREEN: replace the inline `mdsmith.NewSession` in
[doctor.go](../internal/doctor/doctor.go) with a `headroom.Session(root)`
call; keep the proto check ahead of it.

Gate: the new test and every existing `doctor` test pass; `go test
./...` and `mdsmith check .` clean.

## Phase 4: the oracle runs only where its answer is shown

`gatherFleet` computes the headroom signal only for the verbs that
render it, not on every one of the fifteen.

This phase is a proving slice. The RED reproduction fixes the real shape
before the GREEN sites, since routing the signal only to `ready`/`pick`
touches the shared gather.

RED first, in
[gather_test.go](../internal/fleet/gather_test.go). A repository gets a
malformed `.mdsmith.yml`. A plain `Gather` opens no headroom session and
reports no problem. `Gather` with the caller opted in still fails with
the expected problem. That is the call-count proof, with no need to
instrument `Session.Check` itself.

GREEN: a `fleet.Options.Headroom bool` gates `headroomFor`. Only
`gatherFleetWithHeadroom` in [main.go](../cmd/frit/main.go) sets it,
called by `ready` and `pick`. The other thirteen verbs keep the plain
`gatherFleet` and never open the session.

The slice settled the second question in the proving slice's favor. A
probe against the real API shows
`Session.Kinds(path).Rules["max-file-length"].Final` reads `true`, not
`{"max": N}`, for the common no-override case. Recovering the cap would
still mean falling back to the rule's own built-in 300 default — a
second copy of that number, the exact drift the package doc already
warns against. `pad` and `fits` stay exactly as they are, since Phase 6
tests them; only gating shipped. The package doc in
[headroom.go](../internal/headroom/headroom.go) is corrected to say why
it still asks the oracle rather than reading `Session.Kinds` for real,
not that the cap is unreachable.

Gate: the new gather test passes; `orphans`/`board`/`drift` open no
headroom session (proven against a malformed-config repo, both with the
built binary and in the test); `ready`/`pick` still report the same
shortfalls they do today, confirmed against the built binary; `go test
./...` and `mdsmith check .` clean.

## Phase 5: the headroom signal ships its skill text

[plan-pick](../.claude/skills/plan-pick/SKILL.md) names the headroom
column `pick`/`ready` now print, and the `headroom_short` JSON key. An
agent reading a shortfall then knows what it means, and does not claim a
plan it cannot write a phase into.
[plan-new](../.claude/skills/plan-new/SKILL.md) step 7 lists `headroom`
among the doctor checks a new plan must satisfy. Both are edited in
`internal/skills/assets` and regenerated into `.claude/skills`.

RED: the phrasing is asserted where the skills are linted — the assets
pass `mdsmith check .` under the `skill` kind's token budget with the
new lines.

GREEN: edit the two asset `SKILL.md` files. Run `frit skills --force
--via "go run ./cmd/frit"`, the dogfood invocation, to regenerate the
copies. `TestDogfoodCopiesMatchCanonical` stays green.

Gate: this phase ships skill text, so it gates on the claim, not the
copy — build frit, run `frit skills` into a scratch repo, and confirm
the installed `plan-pick`/`plan-new` carry the headroom lines. `go test
./...`, `mdsmith check .` and the dogfood test clean.

## Phase 6: pad and fits carry their own tests

`pad` and `fits` in [headroom.go](../internal/headroom/headroom.go) each
gain a dedicated unit test. `pad`'s trailing-newline branch is driven
red first, per CLAUDE.md's Defensive Code rule.

RED, in [headroom_test.go](../internal/headroom/headroom_test.go):

- `pad` on a source whose last byte is not `\n` inserts the newline, so
  the padded source has exactly the claimed line count; a source already
  newline-terminated is padded without a doubled blank. The unterminated
  case fails before the branch exists — write it to prove the branch.
- `fits` returns true when the padded source passes max-file-length and
  false when it trips it, against a small in-memory session.

GREEN: the branch already stands; this phase pins it. If the RED shows
the branch is wrong — an off-by-one in the count — fix `pad` to match
the assertion.

Gate: the new tests pass and cover both `pad` branches; `go test
./internal/headroom` shows them driving the trailing-newline branch;
`go test ./...` and `mdsmith check .` clean.

## Execution

Tier is per phase. Each is a written assertion — a doctor finding, a
golden, a call count, a skill line, a branch test — so opus settles the
shape and sonnet implements against the assertion.

| Phase          | Design | Implement | Gate that catches a wrong answer                                         |
| -------------- | ------ | --------- | ------------------------------------------------------------------------ |
| 1 skip done    | opus   | sonnet    | doctor drops the two ✅ rows; a live 🔲 plan past reserve still named    |
| 2 one number   | opus   | sonnet    | no_headroom gone from every golden; label reads headroom_short > 0       |
| 3 one session  | opus   | sonnet    | a repo with no .mdsmith.yml scans on defaults, not a failed session open |
| 4 oracle scope | opus   | sonnet    | orphans/board/drift open no oracle; ready/pick shortfalls unchanged      |
| 5 skill text   | opus   | sonnet    | installed plan-pick/plan-new carry the headroom lines, run against frit  |
| 6 helper tests | opus   | sonnet    | pad's trailing-newline branch driven red then green; fits both ways      |

## Acceptance Criteria

- [ ] `doctor` reports no `headroom` finding for a ✅ or ⛔ plan, and
      still reports one for a 🔲/🔳 plan past its reserve
- [ ] `pick`/`ready` never surface a done plan's shortfall
- [ ] The headroom signal is carried as one `HeadroomShort int`;
      `NoHeadroom` is removed from every layer and the JSON
- [ ] `doctor` and `headroom` open the mdsmith session through one
      helper with one missing-config policy
- [ ] `orphans`, `board`, `drift` and the other non-showing verbs no
      longer run the headroom oracle
- [ ] `internal/headroom`'s package doc no longer claims the cap is
      unreachable through `pkg/mdsmith`
- [ ] `plan-pick` and `plan-new` name the headroom column and key, in
      the asset and the dogfooded copy
- [ ] `pad` and `fits` each have a dedicated test, and `pad`'s
      trailing-newline branch is driven red then green
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
