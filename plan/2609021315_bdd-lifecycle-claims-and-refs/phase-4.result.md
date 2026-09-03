---
n: 4
title: Released before the PR merges runs for real
status: "✅"
result: true
summary: >-
  S58 drops `@pending` and runs as a real scenario in
  `cmd/frit/bdd_lifecycle_test.go`: after `planIsDoneAndItsLeaseIsReleased`
  (S53's and S57's own Given, now also registering its repo under
  `w.clones`), a second named machine's own `acquiresTheLeaseForPlan`
  reacquires the released lease at epoch 2, and two Then steps check
  the matrix's own doc-by-argument observable directly — the marker
  reads the new holder's own name, not the old one, and origin's tip
  matches that claim. `go test ./...` and golangci-lint stay clean, and
  the whole `TestFeatures` suite now reports every row of this half —
  S50, S51, S52, S53, S55, S56, S57, S58, S70, S75 — as PASS, none
  SKIP. This is the plan's last phase.
---
## Handoff

**S58 landed exactly as phase-4.md predicted, with no finding.** No
scavenge was needed or driven: the released ref was reacquired in
place by a second, named machine, the same shape
`TestClaimReacquiresAReleasedLease` already proved at the commit-body
level, now proved at the marker level and against a real second
identity instead of the CLI's own single anonymous one. The two edits
this phase made to already-landed code were both additive and
confirmed safe by the full suite staying green: `acquiresTheLeaseForPlan`
now also records the resulting lease on `w.lease` (previously
discarded), and `planIsDoneAndItsLeaseIsReleased` now registers its
own repo under `w.clones[w.holder]` (previously absent). Neither
change altered what S50, S51, S53, S55 or S57 assert; all five stayed
green throughout.

**The plan's own claim-and-ref half of the matrix is now fully
proven.** Every row plan.md named — S50, S51, S52, S53, S55, S56, S57,
S58, S70 and S75 — is a passing godog scenario, none `@pending`, none
SKIP. The two seams plan.md's own Context flagged as possible findings
going in — "no code names plan-gone evidence for S52" and "the
gather's fetch --prune does not by itself move origin/HEAD" — were
both checked, not assumed, across phases 1 through 3, and both hold:
neither ever forced a change to lease-protocol code or to `DefaultRef`.
No finding blocks any row in this half. The evidence-detection wiring
for S52 (deciding *when* to scavenge a plan-gone ref) and the human
process a plan's own release timing depends on for S58 both remain
exactly what plan.md's Context called them: out of this plan's scope,
for a future plan to pick up if it ever needs the automation rather
than only the proven mechanism.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S58'`
reports PASS, not SKIP; `go test ./cmd/frit -run TestFeatures` reports
every row of this half as PASS, none SKIP, and no other section
regressed; `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean; `mdsmith check .` is clean.
