---
n: 3
title: doctor flags a phase whose front-matter n differs from its filename
status: "✅"
result: true
summary: frit doctor now flags a folder-plan phase whose front-matter n disagrees with its filename, on the skewed phase file's own path.
---
## Handoff

Plan 2608310704 closes here. `planmeta.PhaseFilenameMismatches(dir)`
walks a folder plan's `phase-N.md` files, reusing `phaseSpecNumbers`
and `specFileName` the same way `PhasesFromDir` already does, and
reports every one whose front-matter `n` disagrees with the number its
own filename carries. `doctor.checkPhaseNumberSync` turns each
mismatch into a `phase-n-sync` Finding, pointed at the skewed
`phase-N.md` itself, and `scanFile` calls it for every folder plan.

One deviation from the spec: the new planmeta test landed in
[resume_test.go](../../internal/planmeta/resume_test.go), beside
`TestPhasesFromDirAssemblesFromFrontMatterAndExecutionRows`, not
`plan_test.go` as the spec said — that is where `PhasesFromDir`'s own
tests already live, despite `PhasesFromDir` itself sitting in
`plan.go`. The proving fixture also turned up a live bug worth noting:
a skewed phase's mismatched `n` makes `PhasesFromDir` key it by the
wrong number, so it can *also* trip a spurious `execution-row`
finding alongside the new `phase-n-sync` one — both real, both from
the same root cause, exactly the drift this phase closes.

The claim was run against the built binary, not just `go test`: a
scratch git repo with a skewed `phase-2.md` (`n: 5`) got exactly one
`phase-n-sync` finding from `frit doctor --root`; fixing the front
matter cleared it. `doctorCmd.Help()`, `Finding.Check`'s doc comment,
and the `--help` test's check-name list all name `phase-n-sync` now.

`mdsmith check .`, `go test ./...`, `golangci-lint run ./...`, and
`frit doctor` are all clean. This closes plan 2608310704's Goal in
full: the interleaved `## Phases` catalog and the `n`-vs-filename
doctor check.
