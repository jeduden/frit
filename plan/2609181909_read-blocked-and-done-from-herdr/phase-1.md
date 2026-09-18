---
n: 1
title: Blocked and done read as themselves
status: "🔲"
result: false
---
Probe first, since no Go test can. In a real herdr session, put a
Claude pane at a permission prompt and read `herdr agent list` and
`herdr agent explain --json` for it. Do the same for a pane whose agent
has finished its turn. Record the raw status strings and the
`visible_blocker` value in this phase's result. Save each response as a
fixture under `internal/herdr/testdata`. If `list` never emits
blocked, stop and re-plan: the read would need one `explain` per pane,
which is a cost the owner should weigh.

RED, from those fixtures, each failing on today's code:

1. `ParseAgentList` on the blocked fixture yields a pane whose
   `Presence()` is blocked, not unknown. The finished fixture yields
   done. A status string no fixture names still yields unknown.
2. `Presence()` never returns idle for anything but the idle status.
3. `messageSend` on a blocked pane refuses, names the lane and says it
   waits on an approval prompt, and sends nothing even under `--go`.
   `nudgeSend` refuses it the same way. Both still refuse unknown as
   before.
4. `board --json` and `who --json` carry blocked and done in the
   existing status field. Every key stays present.

GREEN: add the two constants and recognise them in `Presence()`. Keep
its default arm returning unknown. Refuse blocked in both send paths
with the named reason. Check every other consumer of `Presence()`. In
particular `askable` decides where the ask advice appears, so decide on
purpose whether a blocked lane is askable. The answer is no while a
send would land in a prompt, so its advice should say to approve or
open the pane instead. Decide done from what the probe showed and
record why. Render both statuses in the tables. Read the skills and
docs for a claim that only working, idle and unknown exist, and correct
it. Document the wider set in the JSON contract.

BDD coverage: add the next free `@C<n>` to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[commands.feature](../../features/commands.feature): a lane whose pane
is blocked shows blocked on the board, and `message --go` refuses it by
name and sends nothing. Bind steps in a section-owned
`cmd/frit/bdd_*_test.go` on the shared world, using the fake herdr
with the probed fixture. No `@S<n>` applies: no claim, takeover or
resume changes.

Gate: the probe's findings are in the result, taken from a real pane.
The new scenario runs without skipping against the built frit. Run
`frit board --json` against a fake herdr serving each fixture and
confirm the status field matches what the RED cases assert. Any skill
line touched is checked against that output, since lint and the dogfood
match pass on a false claim. Then run the full Go tests, lint and
`mdsmith check .`.
