---
n: 2
title: start and pick hand the prompt back without herdr
status: "✅"
result: false
---
On a host with no herdr installed, `start --go` and `pick --go` stop
refusing. They claim, stand the lane up with git as claim does since
phase 1, and hand the composed prompt back for the caller to run in
that lane. Nothing is dispatched, because there is no pane.

**The handoff.** The start document gains a fourth handoff value,
`handed-back`, beside `preview`, `running` and `none`. It means the
escalation ran: `started` is true and the lease is held. But
`prompt_dispatched` is false, `pane` is empty, and `next_action` is
empty, since `prompt` itself is the recipe to run in `lane`. The table
says so in one line, naming the lane. No key is added, so the JSON
contract holds.

**Resumes.** A self-resume from inside a herdr-less lane renews on its
token and hands back again, with no pane to read. A reattach from
outside reuses the existing checkout. herdr's `worktree open` has no
git counterpart to call, since the checkout already stands. A
leftover worktree on the claim branch is reconciled as today. The
check for a live pane on it reads a missing herdr as "no pane",
which is certain, rather than as an unknown.

**RED.** Three sets of tests, each written failing first:

- C16 is rewritten. Under a git-only `$PATH`, `pick --go --json`
  claims the top candidate, plan 8. Its lane stands on origin's claim
  marker, the handoff is `handed-back`, and plan 7 is untouched.
- A new C18: `start 7 --go --json`, then `start --go --json` from
  inside that lane, hands back both times. The second is `resumed`.
- Unit tests come first, red, for the report's handed-back mark and
  its table, for the live-pane check on a missing herdr, and for the
  headless stand-up's fresh, self-resume and reattach paths.

**GREEN.** start's preflight stops refusing a missing herdr and
carries the fact to the execute step instead. The stand-up branches
on it: a fresh lane goes through phase 1's herdr-or-git choice, a
resume reuses its checkout, and the agent, prompt and focus are
skipped. The missing-herdr refusal and its constant are removed, and
nothing else uses them.

**Docs and skill.** In the README, move `start` out of "herdr must be
running". Teach `plan-pick` and `plan-start` the `handed-back`
branch: run `prompt` in `lane` yourself. That is the one case where
the caller runs `/plan-phase`. Drop the plan-pick line about the
missing-herdr refusal, which no verb gives any more.

**BDD coverage.** C16 is rewritten and C18 added, both command-level:
a handoff value, a `--json` shape, one host. The lease side is phase
1's sessionless hold, which S61 already pins, so no `@S<n>` row.

**Gate.** C16 and C18 pass against the built binary. With the
preflight refusal restored, C16 fails. `go test ./...` and
golangci-lint are clean. A hand run of the built frit under a
git-only `$PATH` shows `start --go` handing back. The skills' new
line matches its `--json`.
