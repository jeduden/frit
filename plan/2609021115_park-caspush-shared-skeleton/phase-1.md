---
n: 1
title: casPush, park and Scavenge's delete share one classify-on-failure helper
status: "🔲"
result: false
---
`casPush`, `park`, and Scavenge's delete confirmation, all in
[internal/claim/lease.go](../../internal/claim/lease.go), each
independently carry the same skeleton. Each attempts a git push, and
on failure, re-reads the remote with `remoteHolderErr` to classify
what actually happened. Extract that shared skeleton into one helper.
This is a design task first, a mechanical extraction second — no
behavior changes, so the gate is the existing suite staying green
unchanged.

**Assumes.** This plan depends on plan 2609021114 landing first.
Scavenge's delete confirmation only shares this shape once that plan's
phase 1 replaces its `remoteHolder` read with `remoteHolderErr`.
Before that, only `casPush` and `park` share it — the narrower
duplication a post-close review of plan 2609012210 flagged and judged
worth its own design pass rather than a rushed extraction inside that
plan.

**Design.** The three sites diverge in what they push (a marker CAS
update, a create-only park, a delete). They also diverge in what
"success" does (`syncLocalRef`, nothing, local-ref cleanup), and in
what each failure shape returns (a `(lost, tip, error)` triple, a
typed `error` alone, a typed `error` alone with a different set of
types). The shared part is narrower than the whole function: run one
push, and on failure read `remoteHolderErr` against one ref.

A helper that does exactly that — takes the push's `--force-with-lease`
clause, the refspec, and the ref to re-read, and returns the push's
error alongside the read's answer (`now` and `readErr`) — lets every
caller keep its own success side-effect and its own failure-shape
switch. The "read fault must not fold into absent" step then lives in
exactly one place. Write this shape down in the phase's own notes or
commit message before touching `casPush`. If the three switches turn
out to share more than the read itself, widen the helper — but do not
force a callback-shaped abstraction the third call site was not asked
for.

**GREEN.** Introduce the helper and rewire `casPush`, `park`, and
Scavenge's delete confirmation onto it, each keeping its own
success-path side effect and its own failure-shape switch inline. No
typed error, message wording, or exported signature changes.

**Gate.** `go test ./internal/claim` unchanged and green before and
after — this is a refactor, not a behavior change, so no new red test
is expected; if the extraction cannot be done without one, that is a
sign the design captured a behavior difference and needs revisiting.
Then the full suite and lint.
