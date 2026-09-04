---
n: 3
title: the gather status joins the report model, in table and JSON
status: "✅"
result: false
---
Carry the gather's `Summary` into the report model, so `frit <verb>
--json` and the table both surface how much of the fleet the answer
reflects. The counts are already on `Result.Summary` by construction;
this phase projects them into the document a consumer reads.

**Assumes.** `fleet.Gather` returns a `Summary` on every `Result` —
discovered, read, fetched, problems, elapsed — populated at
[internal/fleet/gather.go](../../internal/fleet/gather.go). Every read
and mutate verb funnels through `gatherFleetOpts` in
[cmd/frit/main.go](../../cmd/frit/main.go), the one production call site,
so the `Summary` reaches every verb from one place. The report model in
[internal/report/report.go](../../internal/report/report.go) builds the
table and the JSON from one document, under the JSON Contract: every key
present, a list `[]` not null, adding a key does not move `Schema`.

**Value.** A consumer reading a verb's output learns the plans and the
problems, but not whether the walk covered the whole fleet or stepped
over half of it. The status the gather already computes — and today only
a terminal sees on stderr — belongs in the document too, so a
`--json` consumer branching on it, and a person reading the table, can
tell a complete answer from a partial one.

**RED.** Decide the field shape against the JSON Contract and pin it with
tests first:

- add the gather status to the shared document so it rides on every
  verb that gathers, keyed under a stable name (for example `gather`
  or `status`), with every count present — discovered, read, fetched,
  problems — and the elapsed time in a fixed, documented form.
- assert the JSON carries the block on a representative verb, every key
  present, with no key nulled, and `Schema` unchanged (adding keys does
  not move it).
- assert the table surfaces the same status — a concise line or footer,
  built from the one model — so table and JSON cannot drift.
- confirm a partial walk (a stepped-over repository, so `Read <
  Discovered`) renders the reduced counts in both renderings.

Each fails today: the report model has no gather status. Commit the red.

**GREEN.** Project `Result.Summary` into the report document at the
shared construction site, so all ~20 verbs carry it without each
changing, and render it once for the table and once for the JSON from
that one model. Do not touch the stderr progress or `Gather` itself.
Update the golden outputs that grow the new block, confirming each change
is the status line and nothing else.

**Guard the edges.** `Schema` stays 1 — a new key, no changed meaning.
Every key is present even on an empty or single-repository fleet; lists
stay `[]`. stdout still carries only the command's output plus this
documented status; stderr's transient progress from Phase 2 is
unchanged. The elapsed time renders deterministically enough for a
golden test, or is normalised there. Every changed function keeps its
unit test.

**Gate.** New tests pass: a verb's `--json` carries the gather status
with every key present and `Schema` unchanged, the table surfaces the
same status, and a partial walk shows reduced counts; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean; the built
`frit <verb> --json` shows the status block and the table shows its line.

Write the handoff to `phase-3.result.md`. Record the field name and shape
chosen and the contract rationale, confirm `Schema` did not move, and
flip `plan.md`'s `status:` to `✅` with `mdsmith fix PLAN.md`.
