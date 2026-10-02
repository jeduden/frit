---
n: 4
title: The takeover answers its code review
status: "✅"
result: false
---
A high-effort review of the PR left seven findings open. Each is fixed
here, RED first where it changes behavior.

1. Liveness: a matured decorated hold with a live agent in a local
   worktree on its branch is vetoed. A legacy marker names no session,
   so the veto reads herdr's own panes. An unreachable herdr is no
   veto, as for a lease.
2. A delete the server refuses while origin still holds the observed
   tip is a fault naming the push error. Only a branch that moved is a
   lost race.
3. Every retired branch is reported in a `retired` list on claim and
   start, each with its rescue. The single scavenged/rescue pair stays
   a refusal's own.
4. The report says what was removed: whether origin's copy was deleted,
   and whether this host's copy still stands. A branch never pushed
   and kept locally reads "kept", never "retired".
5. The lease and decorated tip readers prefer the configured remote's
   copy, then the local branch, then any other remote's.
6. A decorated branch gone from origin and this clone alike is
   skipped, not reported as a lost race.
7. Scavenge and the decorated takeover share one CAS delete, so the
   two can no longer classify the same failure apart.

BDD coverage: the veto is the lease protocol's VETO met through a new
door — herdr's panes rather than a marker's session — so it gets its
own row. S102: a live agent on a matured decorated lane refuses the
claim, and the branch stands. The rest are single-host classifications,
pinned by unit tests.

Gate: S102 fails with the veto disabled, claiming the plan, and passes
with it. `go test ./...`, the 100% coverage gate and golangci-lint
pass; the claim and start JSON golden files carry `retired: []`.
