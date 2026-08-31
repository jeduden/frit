---
n: 3
title: plan-new defaults to a folder plan and headroom retires
status: "✅"
---
## Handoff

Phase 3 landed, closing plan 2608300937. `plan-new` now defaults to a
folder plan with each phase in its own `phase-N.md`, and the
`internal/headroom` signal has retired end to end.

Shipped:

- `plan-new`'s Method, in both the canonical
  [plan-new/SKILL.md](../../internal/skills/assets/plan-new/SKILL.md)
  and its dogfood copy, now makes the folder shape with
  `phase-N.md`/`phase-N.result.md` the default; a trivial single-phase
  plan is the one case that stays flat. `plan.md`'s own frontmatter
  drops `phases:` for a new plan — each phase's result file carries
  its state instead of a ledger.
- `internal/headroom` is deleted outright. Its `Session` opener moved
  into `internal/doctor/doctor.go` as an unexported `openSession`,
  which is all doctor's goal/schema checks ever needed from it.
- Deleted with it: `doctorpkg.Scan`'s `headroomPercent` parameter and
  `checkHeadroom`; the `headroom` bullet in `doctor --help`;
  `repocfg.Config.HeadroomReserve`/`DefaultHeadroomReserve` and the
  `headroom-reserve` template line; `fleet.Options.Headroom` and
  `headroomFor`; `main.go`'s `gatherFleetWithHeadroom` (both `ready`
  and `pick` now call plain `gatherFleet`) and `headroomLabel`'s
  column; `discovery.Plan.HeadroomShort` and
  `report.PlanCard.HeadroomShort`, and its key in every report golden.
  Every test that only asserted the retired behavior went with it;
  `plan-pick`'s `headroom_short` mention is gone from both copies.

Verified: the three RED cases from the phase spec, `go test ./...`,
`go vet ./...`, golangci-lint and `mdsmith check .` are all clean as
of this handoff. All of plan 2608300937's Acceptance Criteria are met.
