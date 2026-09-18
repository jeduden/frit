---
n: 1
title: An ask and its reply, end to end
status: "🔲"
result: false
---
Prove the loop on one host, then fix the test approach later phases
copy. Use the fake herdr the existing message tests use, and a real
local git repository with a lane worktree, so the ask record is
exercised through a real git dir.

RED, in order, each failing on today's code:

1. `message --ask` dry run shows an envelope: the text, a line saying
   a reply is wanted, and the exact `frit reply "<answer>"` command.
   Nothing is sent and no record is written. Under `--go` the fake
   pane receives the envelope whole and one ask record exists.
2. `frit reply "<answer>"` run from the lane worktree, with no plan
   argument, finds the plan from the checkout and stores the answer
   against the pending ask. It takes no `--go`, prompts no pane, and
   touches no ref. With no ask pending it refuses, and says why.
3. `board --json` reports the row's ask as `none`, `pending` or
   `answered`, with the answer text when answered. Every key is present
   and a lane with no ask reports `none`, per the JSON contract.

GREEN: `--ask` on the message verb wraps the text and, when a send
happens, writes the record beside the lane's token. Reuse
`claim.TokenPath`'s placement, keyed so the asker in the main checkout
and the responder in the lane's worktree read the same file. Write it
atomically, as the presence cache does. Add `reply` as its own verb
with an optional selector inferred from the cwd, the way `open` and
`nudge` infer theirs. Carry the ask state on the board row through the
one report model, so the table and `--json` never diverge.

Skill front: `reply` ships with the skill that fronts it, in this
change. Fold it into plan-drive's "Ask directly" for the asker and into
the skill a responder loads, whichever stays under its token budget.
Do not raise a budget or touch `.mdsmith.yml` without the owner's
consent; if the budget forces it, stop and ask. State that `frit reply`
is the one command to allowlist for the round trip. Its example shows
`--json` where an agent branches on the result.

BDD coverage: add `@C13` (or the next free id) to
[command-scenarios.md](../../docs/research/command-scenarios.md) and
[commands.feature](../../features/commands.feature): a supervisor asks
with `--ask --go`, the lane answers with `reply`, and `board --json`
reports answered with the answer text. Bind steps in a section-owned
`cmd/frit/bdd_*_test.go` on the shared world. No `@S<n>` applies: no
claim, takeover or resume changes. This is a single-host command
behavior.

Gate: C13 runs without skipping against the built frit. Run `frit reply`
and confirm it needs no `--go` and leaves refs and panes untouched.
Confirm the skill's example command matches the built output; lint and
the dogfood match pass on a false claim. Then run the full Go tests,
lint and `mdsmith check .`.
