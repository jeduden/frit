# Finding and reaping orphaned work

frit stores nothing about which leases are live; it recomputes state
on every run from the ref list and, where reachable, herdr. `frit
board` shows every outstanding plan with its holder and, for a stale
one, how long its window has sat matured. `frit orphans` reports what
no longer adds up:

| Category    | What it means                                           |
| ----------- | ------------------------------------------------------- |
| unstaffed   | a lease with no worktree behind it                      |
| stranded    | a worktree left standing on a ref that has since landed |
| empty       | a worktree prepared and never started                   |
| prunable    | a worktree whose checkout is already gone               |
| migratable  | a legacy decorated hold, and the id-only ref it maps to |
| stale holds | a held plan whose takeover window has matured           |
| rescued     | a leftover rescue ref, before a blocked park is hit     |

`frit who` reports which lane has a live agent on it, read from
herdr's own pane list. A claim is visible everywhere the remote is
fetched; a running agent, only on its own machine.

## Reaping

`frit reap` acts on the whole table above. It is a dry run by
default and acts only on `--go`. Everything it refuses carries the
reason, and every delete honors the park-before-delete rule of
[the lease protocol](research/lease-protocol.md): unlanded work is
parked to a rescue ref before anything is removed.

`frit reap <selector>` narrows the stranded-checkout row to the one
plan the selector resolves to, the way `yield` and `start` already
narrow their own verb. The unstaffed and rescued rows still sweep the
whole fleet regardless of a selector — narrowing those is later work.
A bare `frit reap`, selector omitted, is unchanged.

A stranded worktree is removed and its branch deleted, but only when
frit's own evidence confirms the branch landed. Ancestry evidence is
tied to the tip, so the delete loses nothing. The ✅ glyph is not:
the branch tip is parked first, so a commit the squash never carried
is moved rather than destroyed, and a park that cannot happen
refuses the whole teardown. An empty or prunable worktree is
removed; its branch is left alone.

An unstaffed lease is dropped through the scavenge, its unlanded
work parked first, but only on abandonment evidence: a matured
takeover window, or a bound session herdr confirms dead. A missing
local checkout is not evidence — the checkout may be another
machine's — so a live lease is refused. Its own lane ends it with
`frit release`, or `frit claim` takes it over once the window
matures. A legacy decorated hold is refused with the id-only ref to
migrate to.

The evidence classes are defined on
[the claiming page](claiming.md) and in the protocol note; the reap
verb was added by
[plan 2608212218](../plan/2608212218_reap-the-orphans.md).
