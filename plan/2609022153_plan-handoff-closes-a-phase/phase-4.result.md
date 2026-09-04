---
n: 4
title: doctor honors the lane it runs in
status: "✅"
result: true
summary: >-
  frit doctor now reads the plan whose lane the cwd stands in from the
  lane's own working copy, so a gap fixed in the lane clears before the
  branch merges — the same narrowing next, show and phase already apply
  via laneOverride. Every other plan still reads the fleet's
  default-branch copy. New doctor.ScanID re-checks one plan from a
  given root; doctorCmd.Run swaps that plan's findings when repo.Name
  matches the current lane; the help text now states the lane read.
---
## Handoff

Landed as scoped, reusing the seam next/show/phase already stand on.

`internal/doctor/doctor.go` gained exported `ScanID(root, planDir, id)`
— it opens the session and lists plan paths exactly as `Scan` does,
keeps only the path whose `leadingIDToken` matches the id, and scans
that. The shared body of `Scan` and `ScanID` is factored into
`scanPaths`, so neither duplicates the session-open-plus-sort loop.

`cmd/frit/main.go` `doctorCmd.Run` reads the cwd once and asks
`fleet.CurrentLane` for the lane's repo, id and root. In the repo loop,
when `repo.Name` equals the lane's repo, `overrideLaneFindings` drops
that id's default-branch findings and appends a fresh
`doctorpkg.ScanID(laneRoot, ...)` — the identical "swap only the plan
the cwd resolves to" narrowing `laneOverride` gives next, show and
phase. `doctorCmd.Help()` now states the lane read outright, closing
the "silent" complaint the blind-agent test raised.

RED was two tests. `TestScanIDChecksOnlyTheNamedPlanFromItsOwnRoot`
(doctor package) pinned that ScanID reports only the named plan's gaps
— it compile-failed until ScanID existed.
`TestDoctorInsideItsOwnLaneReadsTheWorkingTreeCopy` (cmd/frit)
reproduced the surprise verbatim: main carries a plan whose done phase
left no handoff, the lane has written it, yet doctor-from-the-lane
still reported the finding. Its sibling
`TestDoctorOutsideTheLaneStillReadsTheDefaultBranch` pins that the
default-branch read is unchanged from outside a lane.

**One boundary worth stating.** The override swaps only the plan the
cwd resolves to. Standing in plan 2609022153's lane, `frit doctor`
still reports plan 2609021554's handoff gap, because that plan is read
from main and this lane's fix to it has not merged — correct, and the
same boundary next/show/phase keep. This lane's own plan reports clean:
`go run ./cmd/frit doctor` finds zero findings for 2609022153, where at
HEAD it would have read main (which may not even carry this plan yet).

**On the design choice.** The user picked lane-aware over "main-only
but loud" or "both" after a blind-agent test (four agents, two smaller
models) found the main-only behavior unanimously surprising and a
re-fix-loop trap for smaller models. Per-finding provenance labels
(the "both" option) were deliberately left out to keep doctor's output
unchanged in shape; the help text carries the provenance instead.

Verified: `go test ./internal/doctor/... ./cmd/frit/...`, the full
`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`, and
`mdsmith check .` are all clean.

**What's still open.** Task 5 has no phase file yet: point
`plan-phase`'s step 4 at `/plan-handoff` for the close, so the two
skills cannot drift. The plan stays 🔳.
