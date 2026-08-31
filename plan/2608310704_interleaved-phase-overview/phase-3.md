---
n: 3
title: doctor flags a phase whose front-matter n differs from its filename
status: "✅"
result: false
---
Give `frit doctor` a new check, `phase-n-sync`. It mirrors `id-sync`.
One finding per folder-plan phase. Its `phase-N.md` front-matter `n`
disagrees with its own filename's number. The two can drift silently
today, as the plan's Context section describes.

**RED.** Add `TestPhaseFilenameMismatchesFlagsOnlyADivergentN` beside
the phase tests in
[internal/planmeta/plan_test.go](../../internal/planmeta/plan_test.go).
Build a fixture directory. Give it a synced `phase-1.md`, `n: 1`. Give
it a skewed `phase-2.md`, `n: 5`, filename still says 2. Call
`planmeta.PhaseFilenameMismatches(dir)`. It does not exist yet. The
package fails to compile.

**GREEN, planmeta.** Work in
[internal/planmeta/plan.go](../../internal/planmeta/plan.go), beside
`PhasesFromDir`. Add `type PhaseNumberMismatch struct { FileToken,
FrontMatterN string }`. Add `func PhaseFilenameMismatches(dir string)
([]PhaseNumberMismatch, error)`. Reuse `phaseSpecNumbers` for the
filename tokens. Reuse `specFileName` for each path, the same as
`PhasesFromDir`. Read each file with `parsePhaseFile`. Compare its
`Phase.N` to the filename token. Skip a pre-convention file with no
front matter; do not error on it. Walk only `phase-N.md`.
`specFileRE` already excludes the result file.

**RED, doctor.** Add `TestScanFlagsAPhaseNumberMismatch` beside
`TestScanSeesFolderPlansAndProvesIDSync` in
[internal/doctor/doctor_test.go](../../internal/doctor/doctor_test.go).
Build the same kind of folder plan. Give it a synced `phase-1.md` and a
skewed `phase-2.md`, `n: 5`. `Scan` reports no finding for it today.
Assert one finding once GREEN, on the skewed file's own path.

**GREEN, doctor.** Work in
[internal/doctor/doctor.go](../../internal/doctor/doctor.go). Add
`checkPhaseNumberSync(id int64, planRel, dir string) ([]Finding,
error)`. Source it from `planmeta.PhaseFilenameMismatches`. Shape it
like `checkIDSync`: one finding per divergence. Call it from `scanFile`
for every folder plan, `plans.IsFolderPlanFile(rel)`, with `dir =
filepath.Dir(path)`. Call it unconditionally, not gated on
`plan.Phases` being empty — a phase file's own number can drift either
way. Point each finding's `Path` at the skewed `phase-N.md` itself, not
`plan.md`. Add `"phase-n-sync"` to `Finding.Check`'s doc comment. Add
it to `doctorCmd.Help()` in
[cmd/frit/main.go](../../cmd/frit/main.go) too. Add it to the
check-name list
[cmd/frit/doctor_test.go](../../cmd/frit/doctor_test.go)'s `--help`
test already carries.

**Gate.** Build a folder-plan fixture. Give its `phase-2.md` an `n: 5`
front matter; the filename still says 2. `Scan` reports nothing for it
at HEAD. Once GREEN, `Scan` reports exactly one `phase-n-sync` finding,
on `phase-2.md`. A synced fixture stays clean throughout. `go test
./...` is green.
