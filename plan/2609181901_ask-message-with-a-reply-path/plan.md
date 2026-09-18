---
id: 2609181901
title: An ask carries its reply, so silence is no longer the only answer
status: "🔳"
summary: >-
  frit message writes text into a pane and stops. The remedy frit board
  prints for a (dead) lane is that same message, so an unanswered ping
  reads as proof the lane is gone when the agent was never asked to
  answer (issue #198). Give the ask a reply path: message --ask tells
  the agent an answer is wanted and how to give it, a new reply verb
  records the answer as a local write with no --go and no pane send,
  a plan-reply skill pre-approves that one command through its
  allowed-tools, and board and who show whether an ask is pending or
  answered. The (dead) advice stops treating silence as evidence.
model: sonnet
depends-on: []
---
# An ask carries its reply, so silence is no longer the only answer

## Goal

A supervisor can ask a lane's agent a question and learn from frit
whether it was answered. Today an unanswered `frit message` proves
nothing, because the agent was never told a reply was wanted and had
no reply that needed no operator's sign-off.

## Context

**The misread, observed** (issue #198, frit 0.14.0). `frit board`
flags a lane `(dead)` and advises `frit message <id> "what is your
status?"` before yielding. Two lanes so flagged got that message with
`--go`. The text landed in the pane; neither lane replied; the silence
was read as confirming the diagnosis. It confirmed nothing. `message`
writes into the pane's input and carries no expectation of an answer,
so a live session can leave it unanswered indefinitely.

**The follow-up gap** (the issue's comment). A message that spells out
the reply command does get through, and the agent drafts the right
status. But the reply is itself `frit message <target> "<text>" --go`,
an outbound send, and it stopped at the responder's own permission
gate until a human relayed it. Naming the reply is not enough. The
reply must not be a gated send.

**Why the reply is a local record, not a second message.** This is
the repository's main principle: steering is local, coordination is
origin. A live pane and an answer to it are local facts. frit reads
them from the host that has them and never infers them from refs. So
a reply is not another injection into the asker's pane. The responder
writes a small record beside its lane's token, and the asker's frit
reads it. That write touches no pane, no ref and no network, so it
needs no `--go`. That is also what makes it safe to pre-approve.

**Why a project rule approves it.** A skill's `allowed-tools` front
matter was meant to pre-approve the command while the skill runs, and
`plan-reply` still lists only `Bash({{frit}} reply:*)`. Phase 1 checked
that in a real session and it did not hold: the harness prompts before
it loads the skill, and under `claude -p` the Bash step was denied even
after the skill loaded. A project allow rule for the same command
worked. So `frit skills` also writes `Skill(plan-reply)` and
`Bash(<invoke> reply:*)` into the repository's `.claude/settings.json`,
and the skill's `allowed-tools` stays as documentation of the intent.
The rule covers a side-effect-free write, not a send. Three limits are
stated, not hidden. The approval needs the skills bundle installed in
the responder's repository. Claude Code applies a project's allow rules
only in a folder the operator has trusted, so a lane in an untrusted
folder still prompts. A deny rule in the harness still wins.

**What is reused.** Searched and reused:

- `message` — [`messageSend`](../../cmd/frit/dispatch.go) and
  `herdr.Prompt` keep sending the text. `--ask` wraps the text; it adds
  no new send path.
- The skills bundle — `frit skills` already lays skills into a repo
  and substitutes the invocation for `{{frit}}` across each file's
  bytes, front matter included. The approval pattern therefore
  follows `--via`, with no second mechanism.
- The lane token — `claim.TokenPath` puts a file beside the git dir.
  The ask record follows that shape, so the asker in the main checkout
  and the responder in the lane's worktree meet on one file.
- `Ask` on the board row and card — `report.AskCommand` already
  reaches every site that advises asking. Changing it once carries the
  advice everywhere.
- `MessageDoc` and its dry-run/`--go` contract — `--ask` extends it and
  keeps the dry run by default.

Searched and not reused: `nudge` and the presence cache. `nudge`
sends only the phase prompt, and only to an idle lane. The presence
cache is a herdr snapshot. An answer is not presence, and a stale
TTL would misreport it.

**Scope.** One host. `herdr.Prompt` writes through this machine's
herdr, and the record is a local file, so an ask and its reply meet
only when both lanes share a git dir. A reply written on another host
is not read here. Carrying it across hosts is a separate follow-up,
named here so it is not mistaken for done. It would ride the host
transport in
[cross-host-presence](../../docs/research/cross-host-presence.md).

**BDD coverage.** Command-level behavior, one host, no lease race: a
`@C<n>` row per
[docs/development.md](../../docs/development.md)'s executable
scenario matrix. No `@S<n>` applies, since no claim, takeover or
resume changes. Phase 1 reserves the next free `C<n>` against origin's
matrix at execution time; C13 is the next free as of this writing.

## Tasks

1. Phase 1 proves the loop end to end on one host: `message --ask`
   sends an envelope that tells the agent a reply is wanted and names
   the `plan-reply` skill, `reply` records the answer with no `--go`,
   and `board --json` reports the ask pending, then answered. It ships
   `plan-reply`, whose `allowed-tools` documents the approval.
2. Phase 2 makes the approval real: `frit skills` writes the project
   allow rule, merged into an existing settings file, after Phase 1's
   real-session check showed `allowed-tools` alone does not approve.
3. Later phases are specced once Phase 1's handoff shows the real
   shape. Expected: the `board` and `who` tables render the ask state;
   the `(dead)` advice says plainly that silence is not evidence and
   points at `--ask`; the pending-ask state feeds `plan-drive`'s
   ladder.

## Execution

| Phase | Title                                           | Tier   | Gate                                                                                                                                        |
| ----- | ----------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | An ask and its reply, end to end                | sonnet | C13 runs the built frit: ask, reply, answered in `board --json`; a real session replies with no prompt; `go test ./...`                     |
| 2     | The bundle allows the reply in project settings | sonnet | C14 runs the built frit: `skills` writes the reply rule and keeps existing settings; a real session replies with no prompt; `go test ./...` |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                      |
| --- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [An ask and its reply, end to end](phase-1.md)                                                                                                             |
|     | ↳      | An ask now travels to a lane and its answer back, and board --json reports it; the plan-reply skill's pre-approval did not hold under default permissions. |
| 2   | 🔳     | [The bundle allows the reply in project settings](phase-2.md)                                                                                              |
<?/catalog?>

## Acceptance Criteria

- [x] `frit message <id> --ask "<text>"` tells the receiving agent an
      answer is wanted and gives the exact reply command; a dry run
      shows that envelope and sends nothing.
- [x] `frit reply "<text>"` from inside a lane records the answer to
      the latest pending ask, writes no pane, ref or network, and needs
      no `--go`. With no ask pending it refuses.
- [x] `frit board --json` reports each lane's ask as none, pending or
      answered, with the answer text, so an agent branches on a field.
- [ ] The `(dead)` advice says an unanswered ask is not evidence the
      lane is gone, and points at `--ask`.
- [x] A `plan-reply` skill fronts `reply`, and the bundle's project
      settings allow that one command, so a responder with the bundle
      installed replies with no operator sign-off, checked in a real
      session. The skill's example runs against the built frit.
- [x] The envelope names the skill and also gives the raw command, for
      a repository without the bundle.
- [x] One host only: the plan says a cross-host reply is not covered.
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
