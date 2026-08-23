# How claiming works

frit reads many git repositories and shows the plans in them. It
changes exactly one thing: the lease on a plan's work ref. A lease
marks a plan as being worked, recorded on the repository's shared
remote, so other machines see it once they fetch. This page explains
how a lease is made and kept alive, how races and takeovers resolve,
how a fenced lane exits clean, and how to find a lease that needs
attention. The design record behind every rule here is
[the lease protocol note](research/lease-protocol.md).

## The fleet

The fleet is every git repository under one directory, the root, set
with `--root` (or `FRIT_ROOT`, or a config file). Most commands read
the whole fleet; `frit init` writes to one repository you name, and
`frit version` reads nothing.

## The work ref is the lease

A git ref is a name pointing at a commit. frit's hold on a plan is one
ref, `refs/heads/plan/<id>` — the id alone, no slug: `plan/7_shader-unit.md`
claims on `plan/7`. Nothing local — a title, a slug, a machine's own
naming choice — ever reaches the name, so two claimants can never mint
different branches for the same plan.

The ref's tip is the lease token. Every state transition — acquire,
renew, release, takeover, resume, scavenge — is one server-side CAS
(`git push --force-with-lease` with an exact expected old value). The
remote is the sole arbiter; frit never decides holdership from a local
view, and a failure is classified by what the remote says holds the
ref now, never by git's error text.

frit finds holds by listing branches and keeping the ones a
repository's `.frit.yml` `holds` patterns match. The default patterns
are `plan/{id}` (the shape frit mints) and `plan/{id}-*` (see
[Legacy holds](#legacy-holds)). frit drops any branch already merged
into the default branch before it matches, so a finished plan stops
showing as claimed even though its branch still exists.

## The marker

Every transition pushes an empty commit — a marker — that changes no
files. Its trailers are the lease state:

```text
plan 7: claim

epoch:   1
nonce:   3f2a9c1e
holder:  workshop
lane:    /home/you/git/atlas-shader-unit
session: -
base:    3f2a9c1e8b7d4056a1c9e2f0b8d6473a5e1c9f20
```

`epoch` increases on every acquisition — acquire, re-acquire, takeover
— never on a renewal. The current holder's epoch always outranks
whoever it replaced. `nonce` is fresh on every marker, so a deleted
and recreated ref can never reuse a SHA a pending check still expects.

`holder`, `lane` and `session` are for reporting — the board and
`orphans` read them — and never gate a check by themselves. The token
is the tip SHA, persisted locally on acquire and every renewal; that
is what a verb's push CASes against, and what lets a lane resume its
own lease (see [Self-resume](#self-resume)). `base` appears only on a
claim marker: the base commit the acquisition was dated against.

## Work rides the same ref

`frit start` acquires the lease first. Only if that lands does it ask
herdr, in order, to create the worktree, start the agent, bind the
session to the lease, send the prompt, and focus the pane. An agent's
own commits land on top of the marker, each a renewal at the same
epoch — the lease ref and the work ref are the same ref.

## When a plan can be claimed

frit tries to acquire a plan only when all of these hold:

- its status is 🔲 not-started, or 🔳 in progress with no ref holding
  it — that second case is a resume, and frit re-acquires the lease
  for it. ✅ done and ⛔ superseded are refused.
- no ref already holds it — unless its lease has matured, which
  `claim` takes over instead (see [Staleness and
  takeover](#staleness-and-takeover))
- every plan it depends on is ✅ done, in the same repository
- its repository's name is not shared by another checkout under the
  root

If a plan depends on an id frit cannot find, frit counts that
dependency as not done. The refusal then reads "blocked by a
dependency" whether the dependency is unfinished or the id does not
exist. Run `frit show <id>` to tell the two apart.

Passing these checks is not the final word. The acquire still has to
win the push, and another machine can take it first (see [Two machines
at once](#two-machines-at-once)).

`frit ready` lists the plans that pass the status and dependency
checks. `frit pick` lists the same plans in order of how many others
each one unblocks.

## Making the claim

`frit claim <plan>` reads the fleet and finds the plan. If the plan
can be acquired, frit resolves the base commit, builds an empty marker
commit at epoch 1 on it, and pushes the work ref — accepted only if
the ref does not already exist. If the plan cannot be acquired, frit
writes nothing and prints the reason. A third outcome, a hard failure,
is covered in [Failures that are not refusals](#failures-that-are-not-refusals).

A ref whose tip is a release marker is re-acquired instead: the new
claim marker is a child of that release marker, at epoch E+1, so
history is appended, never rewritten. Anything else on the tip is a
live lease, and the CAS is expected to lose.

## Two machines at once

Two machines can decide to claim the same plan at the same time. Each
read its own local view before either pushed, so each thinks the plan
is free. Both build a marker locally and push, CAS expecting the ref
absent.

The remote accepts the first push. The second finds the ref already
there and its CAS fails. The loser re-reads what tip holds the ref
now and reports "lost the race to another machine", naming it when
the marker is readable. This arbitration only holds between machines
pushing to the same remote, and it is the same read-the-tip
classification every transition uses — a renewal that loses reports
[fenced](#fencing-and-yield), a takeover that loses reports the live
holder that beat it.

## Staleness and takeover

An observer records `(ref, tip, first-seen, last-seen, samples)` per
host, adding a sample whenever a fleet-reading verb fetches the tip. A
lease is stale once its tip has sat unchanged for more than
`takeover-window` (T) of the observer's own elapsed time, with no gap
between samples wider than `sample-gap` (S_max). A gap that wide voids
the window instead of counting toward it, so an origin outage resets
every observer rather than triggering a mass takeover on recovery.
Losing the observer's state file only delays a takeover: an absent
record reads as "first seen now".

`frit claim` and `frit start` take a matured lease over: a takeover
marker, epoch E+1, minted as a child of exactly the observed stale
tip. A holder that was merely quiet renews first and wins the CAS. The
takeover loses, re-reads, and reports the live holder instead; it
never retries blindly. A takeover waits `k · T`, not `T`, where `k` is
the number of takeover markers already in the ref's chain. Every
observer computes the same `k` from the chain itself, so two
quiet-but-live agents contending for the same lease damp out instead
of ping-ponging.

### Liveness veto

Before any of that, a live herdr session bound to the lease vetoes the
takeover outright, and renews the lease on the holder's behalf — but
only a positive answer counts. An unreachable host, a dead daemon, or
an unknown session is no veto, and the takeover proceeds; the window
alone decides. A read-only verb never renews.

### Self-resume

A lane whose persisted token matches the work ref's current tip, with
herdr confirming no live session owns that lane, resumes its own lease
immediately — no window consulted at all. A fleet of one is a lane
that just restarted, with nobody else around to renew it or vote for
it. This is what lets it recover as soon as it comes back, rather than
sit locked out by its own staleness window.

## Fencing and yield

Fencing is the CAS itself. The holder is whoever minted the current
tip; a lane's own persisted token is what its next push CASes against.
If the lease moved — a takeover from elsewhere, a hand-edited ref —
that push loses, the lane is fenced, and the refusal names the mover
and offers `yield`:

```text
fenced: the work ref for plan 7 was moved by workshop-2; run yield
```

`frit yield <plan>` is that way out: it pushes the fenced lane's local
divergence to a rescue ref, `refs/frit/rescue/<id>/<holder>/<tip>`
(create-only, since a fenced lane holds no lease to CAS on), tears the
lane's worktree down through herdr, and exits clean. It refuses when
run from the lane that still holds the live lease — yield is for the
fenced, not an alias for `frit release`. `frit next` and `frit show`
list a plan's rescue refs, so parked commits are found again. `frit
orphans` sweeps every repository's leftover rescue refs first.

## When a claim is refused

frit refuses a claim it cannot safely make. A refusal is not a
failure: the command prints the reason and exits 0.

| Reason                    | What it means                                                                   |
| ------------------------- | ------------------------------------------------------------------------------- |
| already held              | a live lease's holder beat this run to the CAS                                  |
| held by a live session    | a bound herdr session vetoed the takeover (renewed on its behalf when it could) |
| already done              | its status is ✅                                                                |
| superseded                | its status is ⛔, replaced by another plan                                      |
| blocked by a dependency   | a plan it depends on is not done, or its id does not exist                      |
| lost the race             | another machine's CAS landed first                                              |
| repository name ambiguous | two checkouts under the root share this repo's name                             |

"already held" and the live-session veto are checked before the
status reasons, so a plan that is both held and done reports the
hold. A 🔳 plan nobody holds is not refused: frit resumes it by
re-acquiring the lease, and the push still arbitrates in case a live
hold does exist.

The last row is a safety stop. frit names each repository by its main
worktree's directory name. If two repositories under the root have
the same directory name, frit cannot tell which one the plan is in,
and could push to the wrong repository. So it pushes to neither and
prints the reason. This blocks claiming every plan in both
repositories. Rename one to fix it.

## Legacy holds

`plan/{id}-*` — the id followed by a slug — is still a hold pattern by
default. A repository that predates the lease protocol keeps reading
its old branches as claims with no flag day. frit only ever mints the
id-only shape now; a repository that narrows `holds` to drop
`plan/{id}-*` stops recognizing its own history of decorated branches.
`frit orphans` lists a decorated hold as migratable, naming the
id-only ref it corresponds to, so the old branches can be retired on
their own schedule.

## Parameters

`takeover-window` and `sample-gap` — T and S_max above — live in a
repository's `.frit.yml`, the same file that declares `holds`, so the
knobs travel with the repository rather than one machine's config. An
undeclared repository keeps the defaults, two hours and thirty
minutes. A value that fails to parse as a duration is a loud error at
the point `.frit.yml` is read, never a silent fall-back to the
default.

## Failures that are not refusals

A refusal exits 0: the plan was simply not this run's to take. A
failure is different. It exits non-zero, prints a plain error, and
writes no report — not even under `--json`. frit fails, rather than
refuses, when:

- the plan selector matches no plan, or more than one
- frit cannot resolve the base commit — the repository has no `main`,
  no `master`, no `origin/HEAD`, and no `base:` set in `.frit.yml`
- `.frit.yml` sets `takeover-window` or `sample-gap` to a value that
  will not parse as a duration
- the push fails for a real reason (see [Two machines at
  once](#two-machines-at-once))

A numeric selector that matches no plan id is retried as a text match
on titles and branches. So `frit claim 42` can resolve to a plan that
merely mentions 42. Pass an id that exists, or run from inside the
plan's worktree, where frit infers the plan from the branch.

## Finding a lease that needs attention

frit stores nothing about which leases are live; it recomputes state
on every run from the ref list and, where reachable, herdr. `frit
board` shows every outstanding plan and its holder, `frit orphans`
reports what no longer adds up, and `frit reap` tears the reported
orphans down. [Finding and reaping orphaned work](reaping.md) covers
the categories and the evidence each teardown requires.

## Landed evidence and scavenge

Merging a work ref's branch into the default branch does not delete
it — the ref still exists on the remote. `claim` and `orphans` do not
wait for a human to notice: landed evidence is enough for `scavenge`
to delete the ref, parking any commits that never merged onto a
rescue ref first. Evidence tied to the observed tip needs no
staleness window. Evidence that is only the ✅ glyph or a missing plan
file additionally requires a matured window, so a renewing lease can
never be scavenged out from under its holder.

The rescue ref is content-addressed by the tip it parks: two parks
from one lane at different tips never collide, and a retry at a
parked tip is a no-op. What still refuses is a ref moved by hand or
forged, holding a different object at that name. `claim`, `release`,
`start` and `yield` warn: fetch, inspect, delete, retry.

This page describes the lease protocol as it ships today. It was
closed by [plan 2608202144](../plan/2608202144_lease-namespace-claims.md)
and [plan 2608211326](../plan/2608211326_lease-protocol-completion.md),
with the reap verb added by
[plan 2608212218](../plan/2608212218_reap-the-orphans.md), the rescue
conflict named by
[plan 2608211936](../plan/2608211936_rescue-conflict-guidance.md), and
made content-addressed by
[plan 2608212011](../plan/2608212011_content-addressed-rescue-refs.md).
A slug in the branch name is not part of the current mechanism; the
rescue ref a blocked park names above is the one manual delete it
now asks for.
