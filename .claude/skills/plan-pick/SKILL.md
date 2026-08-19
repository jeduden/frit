---
name: plan-pick
description: >-
  Find the next plan nobody holds, claim it atomically, and start its
  lane, so two sessions cannot begin the same work. Trigger on "pick
  the next plan", "what should I work on", "claim a plan", "is anyone
  on plan X", "who holds this".
---
# plan-pick

A lane is a worktree working one plan. The claim is a branch ref frit
pushes with `--force-with-lease`, so the ref list is the registry — a
plan's `status:` only reaches the default branch once work merges.
Claim before you start; the push settles the race, a local look does
not.

Run `frit <verb>` (from a source checkout, `go run ./cmd/frit`); add
`--json` to parse.

## Method

1. **Look.** `frit pick -n 5` ranks startable plans by how much each
   unblocks; `frit board` shows who holds each and the live agent.
2. **Verify the top pick.** `frit show <id>` names any unmet
   dependency (a blocked plan is not startable); `frit next <id>`
   names the first unfinished phase and its tier. Then
   `grep -rn "<symbol>" .` — if the artifact already exists, the work
   landed; take the next candidate.
3. **Claim.** `frit claim <id>`. Non-zero exit means someone else
   holds it; the message names the holder — re-run `pick`, take the
   next. Never force.
4. **Start.** Report the branch, lane and evidence. Then run plan-phase
   here, or hand the phase to its own lane with `frit start <id> --go`.

## Notes

- `frit open <id>` focuses a lane's pane; `frit orphans` and `frit
  stale` report lanes that disagree or have gone quiet. Report a stale
  claim, never release another's.
- Which refs count as a claim is declared per repo in `.frit.yml`.
