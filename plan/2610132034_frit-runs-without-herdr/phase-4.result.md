---
n: 4
title: a harness session publishes its lane through its own branch
status: "✅"
result: true
summary: >-
  A session whose harness fixes its branch works its plan in the
  headless lane on plan/<id>, then copies that tip to its own branch
  for the PR; once that branch merges, the plan reads landed through
  existing evidence. No protocol change; C21 proves it, the skills
  teach it.
---
# Phase 4 result

## Handoff

The owner chose to keep work riding the lease ref. A session whose
harness lets it publish only its own branch works its plan in the
lane phases 1–3 stand up with git. It commits on `plan/<id>`, pushes
there (each push renews the lease), then copies the same tip to its
own branch with `git push origin plan/<id>:<branch>` and opens the PR
from it. When that branch merges, the lease tip is in `main`, and the
board reads the plan unheld. A squash merge reaches the same end
through the existing content evidence.

No frit code changed. C21 passed on its first run, which was the
point to prove: the protocol already covers this flow. No `--here`
flag, token-based recognition or marker trailer was built. The
other options are recorded in the plan's Context.

**Verified by.** C21, a command scenario run through godog against
the built frit under a git-only `$PATH`. It starts plan 7 and gets a
hand-back. A commit in the lane is pushed to `plan/7` and copied to
`claude/session`. The board shows plan 7 held. Then `claude/session`
is merged into origin's `main` with a merge commit, and the board
shows plan 7 not held. `go test ./...`, the coverage gate and
golangci-lint are clean.

**Skills and docs.** `plan-phase` teaches the copy step and says to
claim before committing on the harness branch. `plan-pick` and
`plan-start` say to `cd` into `lane` on a hand-back. Architecture's
"Running without herdr" section describes the flow. claiming.md's
yield paragraph names the git teardown.

**Still open.** Whether a cloud session's git proxy lets it push
`plan/<id>` at all is untested. Task 5, an opt-out for a host with
herdr installed, waits on the owner's call.
