---
n: 4
title: doctor honors the lane it runs in
status: "🔳"
result: false
---
Task 4. `frit doctor` scans the fleet's main checkout — `discover.Repos`
groups a repository's worktrees and picks `worktrees[0]`, always the
main working tree. So a finding cleared in a plan's lane keeps
reporting until the branch merges. `next`, `show` and `phase` already
close this gap with `laneOverride`: run inside a plan's lane, they read
that plan's own working copy. doctor does not, and the silent
disagreement is the surprise a blind-agent test found worst for smaller
models. Make doctor honor the lane the same way.

**Assumes.** `fleet.CurrentLane(cwd, git, holdsForRoot)` in
[internal/fleet/current.go](../../internal/fleet/current.go) returns
`(repo, id, root, ok)`: the repository name — the main worktree's
basename, exactly how `discover.Repo.Name` is keyed — the plan id, and
the lane's own worktree root. `laneOverride` in
[cmd/frit/main.go](../../cmd/frit/main.go) already uses it to swap only
the one plan the cwd resolves to, leaving every other plan on the
default branch. doctor reuses the same seam, scanning that one plan's
files from the lane root.

**RED.** Two tests.

- `internal/doctor/doctor_test.go`:
  `TestScanIDChecksOnlyTheNamedPlanFromItsOwnRoot`. A fixture root with
  two plans, one gapped (empty `## Goal`) and one clean.
  `ScanID(root, "plan", <gapped id>)` returns that plan's finding
  alone; `ScanID` for the clean id returns nothing. Fails to compile
  first: `ScanID` does not exist.
- `cmd/frit/doctor_test.go`:
  `TestDoctorInsideItsOwnLaneReadsTheWorkingTreeCopy`, mirroring
  `TestNextInsideItsOwnLaneReadsTheWorkingTreeCopy`. Main commits a
  flat plan, `status "🔳"`, phases `["✅", "🔳"]`, no `## Handoff`
  heading — so doctor on main reports a `handoff` finding. A lane
  worktree on branch `plan/<id>-...` rewrites the plan with a
  `## Handoff` section and commits. Chdir into the lane, run `doctor`,
  assert no finding with `Check == "handoff"` for that id. A sibling,
  `TestDoctorOutsideTheLaneStillReadsTheDefaultBranch`, runs the same
  fixture from the root and asserts the `handoff` finding is present.
  The in-lane case fails first: doctor reads main regardless of cwd.

**GREEN.** Three changes:

- `internal/doctor/doctor.go`: add `ScanID(root, planDir string, id
  int64) ([]Finding, error)`, exported. It opens the session and lists
  `planPaths` the way `Scan` does, keeps only the paths whose
  `leadingIDToken` equals the id, scans each with `scanFile`, and sorts
  the findings the same way. Factor the shared session-open-plus-scan
  body out of `Scan` so both call it rather than duplicating the loop.
- `cmd/frit/main.go` `doctorCmd.Run`: after `discover.Repos`, read the
  cwd and call `fleet.CurrentLane`. In the repo loop, when the run is
  in a lane and `repo.Name` equals the lane's repo, call
  `doctorpkg.ScanID(laneRoot, cfg.PlanDir, laneID)` and replace that
  id's findings: drop every finding whose `ID` equals the lane id from
  the main-scanned set, then append the lane-scanned ones. A missing
  schema on the lane root (`ErrNoSchema`) leaves the main findings
  untouched; any other `ScanID` error is carried as a problem the same
  way `Scan`'s is.
- `doctorCmd.Help()`: add one line to the provenance note — run inside
  a plan's own lane, doctor checks that plan from the lane's working
  copy, the way `next`, `show` and `phase` do; every other plan reads
  the fleet's default-branch copy. Closes the "silent" complaint the
  blind-agent test raised.

**Gate.** `go test ./internal/doctor/... ./cmd/frit/...` covers both
new tests. Full `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` stay clean. `mdsmith
check .` clean.
