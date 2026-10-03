---
n: 3
title: Silence reads as no answer, never as gone
status: "✅"
result: true
summary: >-
  Every ask remedy now reads frit message <id> --ask, the board's ask
  line says no reply is not evidence, and board and who print each
  lane's ask state beneath their tables; who --json carries ask_state
  and answer.
---
# Phase 3 result

## Handoff

The `(dead)` advice no longer sends an operator to a check that
cannot answer. The ask command every remedy shares now carries
`--ask`, so the board's ask line, the discovery card's `ask` and
start's deserted refusal all tell the agent a reply is wanted. The
board's line adds that no reply is not evidence the lane is gone.

Beneath the `board` and `who` tables, a pending ask reads as asked
with no reply yet, saying silence is not evidence; an answered one
prints its answer. One helper renders both, from the fields `--json`
carries. `who --json` now has `ask_state` and `answer` on every lane,
read from the lane's own checkout. A lane on another host, or with no
plan, reads `none`; a record frit cannot read is a problem.

**Verified by.** Unit tests for the command text, the lines, the who
model and its read, committed red first. S91 now runs the remedy as
the refusal names it, `--ask` included, and checks the pane receives
the envelope rather than the bare text. No new scenario row: the
table lines present fields C13 already proves.

**Code review.** A high-effort review found seven things. Fixed:
`who` set a lane's ask by pane id, which a remote and a local lane
can share, so it now marks the lane just added. Two panes on one lane
print their ask line once. `board` and `who` read the record through
one helper. A read-back test now differs from the right answer by
plan id alone. Left as is: an already-asked dead lane shows
both the remedy and its pending state, as this phase specifies; and
`who` runs one git call per planned lane, a cost a cache would trade
for state.

**Second review, owner's calls.** Each ask line names its
repository, `7 (atlas): …`, so two repositories' answers to one id
never read as one another's. An ask is cleared when its lane ends: on
release, on yield, and when a fresh claim or start stands up a new
lane; a resume keeps it. A record that will not go is a warning. The
board finds each repository's ask directory once, not once per plan.
Where an ask can reach is now said where a user meets it: the `--ask`
help, reply's refusal, and the command reference. It reaches only a
lane on this host, through a worktree of one clone. The mdsmith
scaffold's config read moved into its own function with a unit test
that holds for root.

**The ask is decided where it is built.** A board row and a card take
the live lane's attendance — agent, status, and whether it runs on
another host — and decide the remedy themselves. The after-the-fact
swaps on board rows and cards are gone, so a new caller cannot forget
one.

**frit reaches a lane only on this host.** `message` and `nudge`
prompted through this host's herdr by pane id alone, so a lane on
another host could have its text delivered to a local pane sharing
the id. Both now refuse a remote lane, ask or not. Board rows and
cards offer such a lane no ask, and start's deserted refusal names the
host to ask it from.

**Fourth review.** A plan's local lane is now chosen ahead of a stale
remote pane on the same plan, so message and nudge no longer refuse a
lane they can reach. Release clears the ask in the lane's own clone,
where message recorded it. Reply's refusal and the `--ask` help no
longer blame a separate clone: the ask is recorded in the lane's own
clone, so only `board` misses an answer from one. Left open: `open`
still focuses a remote lane's pane through this host's herdr, and an
ask stays pending when its lane ends where no clearing verb runs, as
in a takeover from another host or a reap.

**Plan close.** Every acceptance criterion is met. What stays out of
scope, as the plan said from the start: an ask to a lane on another
host is refused, since its reply would land where this host never
reads.
