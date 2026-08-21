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
settles who starts a plan. Run `frit <verb>` (source checkout: `go run
./cmd/frit`); add `--json` to parse.

## Method

**`frit pick --go`** does the whole pick in one verb. It ranks the
startable plans by unblock weight, takes the top, verifies deps, mints
the atomic claim, and stands the lane up. It resumes an unheld
in-progress plan when nothing fresh is startable, and takes the next
candidate when a claim loses its race. The branch, lane and model are
the plan's — report them, never ask. `nothing startable` means nothing
was claimed.

`frit board` shows who holds what; `frit --help` lists the rest.
