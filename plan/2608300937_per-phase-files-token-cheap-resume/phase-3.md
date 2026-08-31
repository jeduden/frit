---
n: 3
title: plan-new defaults to a folder plan and headroom retires
status: "✅"
---
`plan-new` now writes every plan as a folder,
`plan/<id>_<slug>/plan.md`. Each phase gets its own `phase-N.md` spec.
No more appending `## Phase N` sections to `plan.md`. `plan.md` no
longer grows with phases. So headroom's question — is there room for
another phase section — has no answer left to give. The signal
retires: the `internal/headroom` package,
doctor's `headroom` check, the `headroom-reserve` config key, the
`HeadroomShort` field carried through discovery, fleet and report, the
CLI wiring in `ready` and `pick`, and the mentions in the `plan-new`
and `plan-pick` skills.

RED:

1. [internal/skills/skills_test.go](../../internal/skills/skills_test.go)
   gains `TestPlanNewDefaultsToFolderPlanWithPhaseFiles`. The canonical
   `assets/plan-new/SKILL.md` must name `phase-1.md` as Phase 1's own
   file, not a `## Phase N` section in `plan.md`. It must not mention
   `headroom`. It fails today: the skill still writes phases inline,
   and step 7 still lists the `headroom` finding.
2. [cmd/frit/doctor_test.go](../../cmd/frit/doctor_test.go)'s
   check-list test drops `"headroom"` from the wanted set and gains
   `assert.NotContains(t, help, "headroom")`. It fails today —
   `doctorCmd.Help()` still documents the check.
3. [cmd/frit/discovery_test.go](../../cmd/frit/discovery_test.go)
   replaces `TestPickCarriesTheHeadroomSignal` with
   `TestReadyCarriesNoHeadroomColumn`. The same padded fixture plan's
   row, in a built `ready` table, carries no shortfall column. It
   fails today — the column still prints `-19`.

GREEN, deleting `internal/headroom` outright:

- Fold its `Session` opener into
  [doctor.go](../../internal/doctor/doctor.go) as an unexported
  helper — the one piece doctor still needs, for the goal and schema
  checks `sess.Check` already runs.
- Drop `headroomPercent` from `Scan`/`scanFile`; delete
  `checkHeadroom`; drop the `headroom` bullet and its explanatory
  sentence from `doctorCmd.Help` in
  [main.go](../../cmd/frit/main.go).
- Delete `HeadroomReserve`/`DefaultHeadroomReserve` from
  [config.go](../../internal/repocfg/config.go) and the
  `headroom-reserve` line from
  [template.go](../../internal/repocfg/template.go).
- Delete `Options.Headroom`, `headroomFor` and the now-unused
  `internal/headroom` import from
  [gather.go](../../internal/fleet/gather.go); delete
  `gatherFleetWithHeadroom` from `main.go`, pointing `readyCmd.Run`
  and `pickCmd.Run` at plain `gatherFleet`; delete `headroomLabel` and
  drop its column from `printReady`'s rows.
- Delete `HeadroomShort` from
  [discovery.go](../../internal/discovery/discovery.go) and
  [report/discovery.go](../../internal/report/discovery.go), and its
  golden-fixture line in
  [golden_test.go](../../internal/report/golden_test.go).
- Delete every test this leaves behind, asserting a removed behavior:
  doctor's three headroom tests plus its two now-unused padding
  helpers, repocfg's three config tests and one template test, and
  [gather_test.go](../../internal/fleet/gather_test.go)'s four
  headroom tests plus its `Options{Headroom: true}` call sites.

Rewrite `plan-new`'s Method in both the canonical
[plan-new/SKILL.md](../../internal/skills/assets/plan-new/SKILL.md)
and its dogfood copy,
[.claude/skills/plan-new/SKILL.md](../../.claude/skills/plan-new/SKILL.md).
Step 1 makes the folder shape with
`phase-N.md`/`phase-N.result.md` the default, not a companion special
case. Step 3 has Phase 1 write `phase-1.md`, under the `phase-spec`
kind's freeform-prose shape, instead of a `## Phase N` section. Step 4
drops `phases:` from the frontmatter fields to write — state lives in
each phase's own result file, not a ledger — and names only
`plan.md`'s own sections (`## Goal`, `## Context`, `## Tasks`,
`## Execution`, `## Acceptance Criteria`). Step 7 drops the `headroom`
bullet. Drop the `headroom_short` mention from `plan-pick`'s Survey
section, both copies. Retitle the Notes bullet on a single-phase
trivial plan to name the flat, no-folder shape it keeps.

Gate: the three RED cases pass. A built `frit doctor --help`, and a
built `frit ready`/`frit pick` against a fixture repo with a padded
plan, carry no headroom text. `TestDogfoodCopiesMatchCanonical` passes
on the rewritten skills. `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` stay clean.
