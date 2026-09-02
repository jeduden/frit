---
n: 1
title: Scavenge's post-delete confirmation stops reading unreadable as gone
status: "🔲"
result: false
---
Make `Scavenge`'s delete-confirmation step in
[internal/claim/lease.go](../../internal/claim/lease.go) tell an
unreadable remote apart from a confirmed-gone ref. Drive it red/green
with a scripted runner.

**Assumes.** `remoteHolderErr` already separates a read fault from an
absent ref. Plan 2609012210 used that same split to fix `park`'s
classification of a failed push. `UnconfirmedPushError` models a push
that failed with a confirmation read that failed too. Its wording
("push failed and could not be confirmed either way … it may have
landed") is specific to a landing push and does not fit a delete, so
this phase mints a sibling rather than reusing it verbatim. The
scripted-`gitwt.Runner` pattern in
[internal/claim/park_test.go](../../internal/claim/park_test.go) and
[internal/claim/caspush_test.go](../../internal/claim/caspush_test.go)
drives `Scavenge` the same way: a chosen delete-push error, a chosen
`ls-remote` read answer.

**RED.** Two unit tests on `Scavenge`, through a scripted runner whose
delete push fails.

1. The confirmation read fails too (a stalled or dropped connection
   took out both calls): the returned error reports the delete could
   not be confirmed, carries both the delete's and the read's errors
   via unwrap, and the local ref is left untouched — deleting it on an
   unconfirmed read could destroy the local copy of a lease the remote
   still carries.
2. The confirmation read cleanly answers *still present*: unchanged
   from today — the delete's own error, and the local ref untouched.

**GREEN.** Replace `remoteHolder(repoDir, opts.Remote, ref, run) != ""`
at the delete-confirmation site with `remoteHolderErr`. A read fault
returns a new typed error (e.g. `UnconfirmedDeleteError`, `PlanID`,
`Ref`, `Err` fields, `Unwrap` returning `Err`) wrapping both the
delete's and the read's faults, and the local ref is not touched. A
confirmed-present ref keeps today's wrapped delete error. A
confirmed-gone ref keeps today's local-ref cleanup unchanged.

**Gate.** `go test ./internal/claim` red first, then green; every
existing `Scavenge` test (idempotency, foreign-rescue refusal,
moved-tip refusal, the unreadable-remote read at the top of the
function) stays green unchanged; then the full suite and lint.
