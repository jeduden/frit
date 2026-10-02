---
n: 3
title: Silence reads as no answer, never as gone
status: "🔳"
result: false
---
The issue's own ask: the advice a `(dead)` lane gets must not send an
operator to a check that cannot answer. Phases 1 and 2 built the ask
and its pre-approved reply. This phase points every piece of advice at
it, and shows its state where a person reads.

RED, in order, each failing on today's code:

1. `report.AskCommand(<id>)` reads
   `frit message <id> --ask "what is your status?"`. The board's ask
   line, the discovery card's `ask` and start's deserted refusal carry
   it, since all three already build on it.
2. The board's ask line says plainly that no reply is not evidence the
   lane is gone.
3. Beneath the `board` table, a row whose ask is pending reads as
   asked with no reply yet, again saying silence is not evidence. A
   row whose ask is answered prints the answer. A row never asked
   prints nothing, so a quiet board pays nothing extra.
4. `who --json` carries `ask_state` and `answer` on every lane, read
   from the lane's own checkout. A lane on another host, or one whose
   branch names no plan, reads `none`. A record frit cannot read is a
   problem in the document. The `who` table prints the same lines
   beneath it as `board`.

GREEN: one helper renders the ask lines for both tables, from the
report model's own fields, so the table and `--json` never diverge.

BDD coverage: no new row. The `(dead)` advice is pinned by the
cross-layer scenario that already names the ask command; its step
text follows the new command. The table lines are presentation of
fields C13 already proves.

Gate: unit tests for each RED item; the cross-layer scenario passes
against the new command; `go test ./...`, lint and `mdsmith check .`.
