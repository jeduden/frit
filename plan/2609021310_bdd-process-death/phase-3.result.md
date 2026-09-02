---
n: 3
title: The scavenge and unwind rows S8, S9, S12 and S13 run for real
status: "✅"
result: true
summary: >-
  S8, S9, S12 and S13 drop `@pending` and run as real scenarios,
  driving `internal/claim`'s exported API directly the way phase 1's
  S7 drove `resetWindow`: a released lease leaves a release marker on
  origin and deletes nothing (S8); a scavenge over pushed unlanded
  work parks it to a rescue ref that carries the tip before deleting,
  so the work is never lost (S9); a scavenge over work that has since
  squash-landed on origin's default branch parks nothing and still
  deletes the ref, with no window seeded (S12); and a plan marked done
  on its own branch, origin's default branch untouched, still reads
  unlanded, because `claim.WorkLanded` judges against `origin/main`
  alone (S13). The whole process-death section now runs: `go test
  ./cmd/frit -run TestFeatures/S` reports S1..S13 all PASS, none SKIP,
  and no `@pending` remains in `features/process-death.feature`.
---
## Handoff

**The plan is complete.** All thirteen process-death rows run as
executable godog scenarios; the section that opened `@pending` in its
entirety now proves every promise it makes. `go test ./cmd/frit -run
'TestFeatures/S'` runs S1..S13 as PASS with no SKIP, the bijection
gate (`internal/scenario`) stays green, and `go test ./...`, `go tool
-modfile=tools/go.mod golangci-lint run` and `mdsmith check .` are
clean.

**No finding.** None of the four rows exposed a gap between the
matrix's promise and the code — unlike phase 2, where S4's "RESUME on
the same host" named a path that does not exist. Each of S8, S9, S12
and S13 asserts exactly what `internal/claim` already does, and no
assertion was weakened to reach green.

**One fixture note worth carrying to the sibling BDD plans.** The
squash-landed shape (S12) and the branch-only status flip (S13) both
turn on landed content being judged against `origin/main`, not a
local view. The step that squash-lands copies the branch's own work
file straight onto the default branch (`git checkout <branch> -- w.txt`
then commit and push) rather than re-typing the content, so the
landed check reads a true no-op however the reused
`pushesAWorkCommit` step happened to write it. Any later landed-
evidence row (the sibling `2609021316` plan owns S54, S83, S84, S85)
can reuse that same copy-from-branch trick instead of hardcoding
content that has to be kept in step with the work-commit fixture.

**What the reused steps bought.** S9, S12 and S13 all reuse phase 1's
`"([^"]+)" pushes a work commit on the lane`, and S8, S9, S12, S13
all reuse the lease world's `"([^"]+)" holds the lease for plan (\d+)`.
The only genuinely new vocabulary this phase added was the four
teardown verbs — release, scavenge, squash-land, mark-done-on-branch
— plus their read-backs; every set-up step already existed. That is
the payoff of the section's shared world: a teardown row stands on
whatever a hold-and-push row already built.
