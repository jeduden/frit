---
n: 2
title: A lane with no worktree and no pane
status: "🔲"
result: false
---
Decide first; the owner signs off on each answer below before any
code. Record the answers in this spec, then write RED from them.

The ask: `frit claim <id> --here` (or `--no-lane`) mints the lease
and records the caller's own checkout as the lane, with no worktree
and no pane. A cloud agent session has no herdr, and the harness fixes
its branch, so today it cannot take part in the claim protocol at all.

**1. How the lane is recognised.** Resume, release and yield from
inside a lane find their plan by matching the checkout's branch
against the `holds` patterns, `plan/<id>` by default
([current.go](../../internal/fleet/current.go)). A harness branch such
as `claude/<name>` names no plan. Options: find the lane by its
persisted token, which is already keyed by plan id in the checkout's
git dir, with the id taken from the selector; or widen `holds` per
repository. The token route needs no configuration and is the likely
answer.

**2. Where the work lands.** Today `plan/<id>` is both the lease and
the work branch, and its ancestry into the base is landed evidence. A
herdr-less lane commits to the harness branch, so the lease ref only
ever carries markers and never merges. Landing is then read from the
status glyph alone — the squash-merge shape `scavengeGlyph` already
handles, which waits for a matured window. Confirm that is acceptable,
or name a second evidence source.

**3. Who renews.** No herdr session can be bound, so no live-session
veto protects the hold; only the staleness window does. Something must
beat. The likely answer: the agent re-runs the same claim, which
resumes on the token and renews. A skill line would then say when.

**4. Teardown.** `yield`, `reap` and `orphans` read the marker's lane
as a worktree frit may remove. A herdr-less lane is the session's own
checkout, perhaps the primary clone (plan 2608192322 keeps a claimed
lane off the shared clone for this reason). The marker must say the
lane is a caller's checkout, so no verb ever removes it. Decide the
trailer, and how board and `--json` show it.

**5. The surface.** The flag's name, and whether `start` and
`pick --go` take it too. A herdr-less start could dispatch nothing, so
claim alone may be enough. It skips phase 1's herdr gate rather than
removing it.

BDD coverage: this changes the lease protocol — a hold with no
worktree, renewed without herdr, never torn down — so it needs
`@S<n>` rows. Pick them against origin's
[lease-protocol.md](../../docs/research/lease-protocol.md) at
execution time, per [development.md](../../docs/development.md). Any
new refusal wording also gets a `@C<n>` row.

Gate: the decisions above recorded here. The built frit, under a
`$PATH` with no herdr, claims from a checkout on a non-plan branch,
resumes and renews on its token, and releases. `reap` and `yield`
leave that checkout in place. `go test ./...` and golangci-lint pass.
The shipped skill names the flag.
