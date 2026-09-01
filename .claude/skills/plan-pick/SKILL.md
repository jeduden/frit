---
name: plan-pick
description: >-
  Find the next plan nobody holds, claim it atomically, and start its
  lane, so two sessions cannot begin the same work. Trigger on "pick
  the next plan", "what should I work on", "claim a plan", "is anyone
  on plan X", "who holds this".
---
# plan-pick

The claim is a ref frit force-pushes, so the push — not a local look —
settles who starts a plan. Script against `--json`; a table is for a
person's eyes.

## Method

**`go run ./cmd/frit pick --go`** does the whole pick in one verb. It ranks the
startable plans by unblock weight, takes the top, verifies deps, mints
the atomic claim, and stands the lane up. It resumes an unheld
in-progress plan when nothing fresh is startable, and takes the next
candidate when a claim loses its race. The branch, lane and model are
the plan's — report them, never ask. `nothing startable` means nothing
was claimed.

`prompt_dispatched: true` means the phase is already running in
`pane`. Report the pane and stop there. Never invoke `/plan-phase`
yourself — a second run puts two runners on the same lane.

## Survey first

To look before claiming, without minting a hold:

- `go run ./cmd/frit ready --json` lists the plans startable now — deps done,
  nobody holds.
- `go run ./cmd/frit find <text>` searches plan titles and summaries to resolve
  a plan by a fragment.
- `go run ./cmd/frit board --json` shows who holds what.
- `go run ./cmd/frit --help` lists the rest.
