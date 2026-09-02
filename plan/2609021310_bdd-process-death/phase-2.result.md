---
n: 2
title: The verb-level rows S3, S4, S5 and S6 run for real
status: "✅"
result: true
summary: >-
  S3, S4, S5 and S6 drop `@pending` and run as real scenarios, this
  time driving `cmd/frit`'s own `claim` and `board` verbs through
  `run` and `--json` rather than the raw lease API: a lane that
  already persisted a token from a prior renewal resumes on retry, no
  window read; a claim killed before its worktree ever stood up is
  refused on an immediate retry — not resumed, since no token was
  ever persisted anywhere — and only becomes takeable once the window
  matures; `board` shows a held plan with no agent when nobody is on
  it; a matured hold whose bound session herdr positively confirms
  empty is taken over, the veto's own query path finding nothing to
  protect rather than never running at all. Each `Then` reads
  `report.ClaimDoc`'s `Claimed`/`Resumed`/`Refused` or
  `report.BoardDoc`'s per-plan `Held`/`Agent`, decoded by `emit`'s own
  pattern, and the takeover claim also confirms the minted tip's
  marker actually says `takeover`, not merely that the run reported
  success.
---
## Handoff

**The finding phase-2.md itself already names, restated.** The
matrix's S4 cell promises "RESUME on the same host" for a claim
killed before its worktree exists. No code path delivers that: every
resume door — `ownToken`'s in-lane check and `laneTokenResumeTip`'s
reattach — requires a token already persisted from inside a real
worktree, and a claim killed before `standUpClaimWorktree` ever ran
has stood up no worktree, so it persisted no token on any host,
including its own. This scenario asserts the true outcome — refused
on retry, takeable only once the window matures, same as any other
holder — rather than the more optimistic matrix prose. Correcting the
cell itself is outside this plan, which changes no verb.

**Two verb-level gotchas.** Phase 1's raw-lease-API rows never hit
either; worth knowing before a future row runs a verb the same way.

- A second machine's clone must sit under its own root, one directory
  down (`root/atlas`), the same shape `claimableRepo` gives the
  first. `cloneAgain`'s bare clone sits directly in its own temp
  directory; building `--root` from its parent walks straight into
  the first machine's tree too, and `claim` refuses with "matches 2
  plans". `cloneRepoIntoRoot`, added this phase, is the fix — reuse
  it for any future row that runs a verb, through `run`, as a second
  machine.
- Any scenario that actually runs `claim` needs `herdrReturningWithWorktree`,
  not the bare `herdrReturning` phase 1's rows never needed: `claim`
  always tries to stand the worktree up after minting or taking over
  the lease, and a `worktree.create` call the fake cannot answer
  unwinds the lease it just won, reporting a refusal that looks like
  the row failed for the row's own reason when it was really the
  fixture's.

**No row needed a step the four together didn't already cover** —
unlike phase 1, where S1 and S2 each needed their own Given the lease
world lacked, S3's token-persisted worktree and S4's never-stood-up
one cover the shapes S5 and S6 both reuse ("holds the lease for
plan", reused as-is, is enough for S5; S6 only adds a session to the
same marker).

**What S8, S9, S12 and S13 will need.** All four are `Scavenge`- and
landed-evidence-shaped, not resume- or veto-shaped, so none needs
`claim`, `board` or herdr at all — they read `internal/claim`'s
`Scavenge`, `ParkUnlanded`, `HasUnlanded` and `WorkLanded` directly,
the way phase 1's S7 drove `resetWindow` directly rather than through
a verb.

- S8 (unwind's remote delete fails → a release marker stands, never
  a dangling hold) needs a way to make the CAS delete itself fail
  while the read that classifies it still succeeds — a fake remote,
  or a broken `origin` URL swapped in after the delete's pre-read,
  since `gitwt.Exec` runs real git and there is no injectable
  transport seam today.
- S9 (unwind deletes a branch with pushed work → impossible) is a
  negative claim about `ParkUnlanded`'s park-before-delete rule:
  a scenario that drives an unwind over a tip carrying real work and
  asserts the rescue ref holds it, never that the work is simply
  gone.
- S12 (killed after merge, before status flip → scavenge acts on
  landed evidence, no window) needs a squash-merged base — content
  landed via `landedByContent`/`git merge-tree`, never an ancestor —
  the same fixture shape `claim_test.go`'s existing landed-evidence
  tests already build.
- S13 (status flipped on branch, not merged → evidence reads only
  origin's default branch) needs a plan file edited to `✅` on the
  hold branch itself, with origin's default branch left at its real
  status, and an assertion that a read verb still reports the branch
  status, not the glyph.

All tests are green:
`go test ./cmd/frit -run 'TestFeatures/S(1|2|3|4|5|6|7|10|11):'`
reports nine PASS, none SKIP; `go test ./...`, `go tool
-modfile=tools/go.mod golangci-lint run` and `mdsmith check .` are
clean; `go test ./internal/scenario` (the bijection gate) stays
green.
