# How claiming works

frit indexes and displays. It owns exactly one mutation: the claim. A
claim is how frit says "this plan is mine to work now." It holds across
every machine that shares the same git remote.

This document reads top to bottom. It covers what a claim is, where the
work goes, when a plan can be claimed, how the lease is minted and how a
race resolves, and how a claim is found stale and let go.

## A claim is a git ref

A hold on a plan is a branch, `plan/<id>-<slug>`. The slug comes from the
plan file name, after its id prefix. The branch points at an empty marker
commit. That commit carries the same tree as the base it was dated
against, so the claim touches no file. The ref is the whole point; the
commit exists only to carry it.

The branch name is derived, never stored. So the name frit writes is the
name its hold patterns read back: a lease frit mints is a hold frit
finds. Which names count as a hold is declared per repository, in its
`.frit.yml`. Refs already merged into the default branch are excluded, so
landed work never reads as a live claim.

The claim is minted in [internal/claim](../internal/claim). The verbs
that call it are `frit claim` and `frit start`.

## The hold branch is the work branch

The claim does not sit on a branch of its own, beside the work. The hold
branch is the branch the work lands on.

`frit start` mints the claim, then hands the same branch name to herdr as
a worktree — `worktree create --branch plan/<id>-<slug> --base <base>`.
The checkout stands on the hold branch. The marker is that branch's first
commit, and the work builds on top of it. So the claim and the work share
one branch: the ref that holds the plan, and the commits that do it, are
the same line of history.

That gives a lane two halves under one name. One half is the hold ref
that claims the plan. The other is the worktree standing on it. frit
joins them by the plan id each branch resolves to: both the ref and the
worktree's branch are matched against the repository's hold patterns, and
the id each yields is the join key. So a claim and its checkout read as
one lane, even when the ref is a remote-tracking copy and the worktree is
on the local branch. When the two halves disagree, the lane is orphaned —
and that is what stale detection looks for, below.

## When a plan is startable

A claim is only minted for a plan that is startable. A plan is startable
when three things are all true:

- **not yet begun** — its status is 🔲, not in progress and not done
- **held by nobody** — no hold ref claims its id
- **every dependency done** — each plan it depends on is ✅, in the same
  repository

A dependency that resolves to no known plan is treated as unmet: an edge
frit cannot confirm is done is not assumed to be. This is the one
question the whole index exists to answer, and it cannot be answered from
a single file — it needs every plan's status and every dependency edge at
once.

`frit ready` lists the startable plans; `frit pick` ranks them by how
much each one unblocks. Claiming is allowed on exactly this set: `claim`
and `start` mint a lease only for a startable plan, and refuse anything
else.

## The claim lifecycle

`frit claim <plan>` walks the fleet once. It resolves the selector to a
single plan, checks the plan is startable, and mints a lease if it is.
Otherwise it refuses, and writes nothing.

```mermaid
flowchart TD
    A[frit claim plan] --> B[gather the fleet once]
    B --> C[resolve the selector to one plan]
    C --> D{startable?}
    D -->|held, done, blocked| E[refuse and write nothing]
    D -->|repo name ambiguous| E
    D -->|yes| F[mint the lease]
    F --> G{push accepted?}
    G -->|remote ref was absent| H[claim held: ref on the remote]
    G -->|remote ref already existed| I[lost the race: roll ref back]
```

`frit start` is the same claim, followed by the handoff to herdr: the
worktree, the agent at the plan's tier, and the pane. If any of those
fail to come up, the claim that was already pushed is released. A
half-built lane never lingers as an abandoned hold.

## Minting a lease

Minting is five git commands, run inside the plan's repository. The base
is resolved to a commit and its tree. An empty marker commit is built on
that base. The local hold branch is pointed at the marker. The marker is
then pushed to the remote, under the hold branch's name.

```mermaid
sequenceDiagram
    participant F as frit
    participant G as local git
    participant R as remote
    F->>G: rev-parse base — resolve baseSHA
    F->>G: rev-parse the base tree
    F->>G: commit-tree — empty marker on baseSHA
    F->>G: update-ref — point the hold branch at the marker
    F->>R: push --force-with-lease, expecting the ref absent
    R-->>F: accepted — the ref did not exist
```

The push is the arbitration. It uses `--force-with-lease=<ref>:` with an
empty expected value. That tells the remote the ref must not already
exist. The remote checks this server-side, and rejects the push if the
ref is there. The check is what makes a hold atomic across machines. Git
offers no other atom, so the claim is built on the one it does.

## Racing two machines

Two machines can resolve the same plan as startable at the same moment.
Each gathered its own view before either pushed. So both build a marker
locally, and both push. The remote applies the lease check to each push
in turn. The first to arrive creates the ref and wins. The second finds
the ref already there, and is rejected.

```mermaid
sequenceDiagram
    participant A as machine A
    participant B as machine B
    participant R as remote
    A->>R: push --force-with-lease, expect ref absent
    B->>R: push --force-with-lease, expect ref absent
    R-->>A: accepted — the ref is created
    R-->>B: rejected — the ref already exists
    Note over B: ErrLostRace — roll the local ref back
```

The loser rolls its local ref back. The claim is all-or-nothing, so a
retry starts from a clean state. A lost race is reported, not crashed on.
It is the one expected non-fatal outcome. frit tells it apart from a real
git fault — an unreachable remote, a rejecting hook — by the remote's
rejection signature. Only a ref that already exists reads as a lost race.
Anything else is a fault, and is surfaced as one.

## When a claim is refused

A refusal is not a failure. It is the honest answer that the plan was not
this run's to take. The command still exits clean. The first rows below
are the inverse of startable; the last two are a race lost and a fleet
that cannot be read cleanly.

| Reason                    | What it means                            |
| ------------------------- | ---------------------------------------- |
| already held              | a lane already claims the plan           |
| already done / superseded | the plan's work is finished or replaced  |
| already in progress       | the plan is being worked                 |
| blocked by a dependency   | an unfinished dependency comes first     |
| lost the race             | another machine won the push             |
| repository name ambiguous | two checkouts share the plan's repo name |

The last row is a fail-closed guard. The fleet keys every plan by its
repository's basename. So two checkouts under the root sharing one
basename cannot be told apart. Rather than mint a lease into whichever
the walk reached last, the gather withholds the coordinate and the claim
refuses, naming the collision. Renaming one repository resolves it.

## Detecting a stale claim

A claim is a ref, and a ref outlives the work if nothing clears it. frit
never stores whether a hold is live; it re-derives that each run, by
joining the claim refs to the checkouts standing on them.

Landed work clears itself first. A hold whose branch has been merged into
the default branch is excluded before anything else is counted. So
finishing a plan retires its claim with no extra step, and a merged
branch left undeleted never reads as an active hold.

Two verbs find the claims that outlived their work:

| Verb                  | Finds                                          | The stale claim it catches                                                                                                                         |
| --------------------- | ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `frit orphans`        | lanes whose two halves disagree                | `claimed, no checkout` — a hold ref with no worktree behind it: minted but never stood up, or the checkout was cleaned away while the ref lingered |
| `frit stale --days N` | worktrees whose branch has not moved in N days | a lane whose work has gone idle behind a live claim                                                                                                |

`frit who` reads herdr for which lane has a live agent. It sharpens
`stale`: an idle branch with an agent still attached is paused, not
abandoned, and the report says so. To let an orphaned claim go, delete
the hold ref both locally and on the remote — the remote copy is the
authoritative one other machines read, so a local delete alone leaves the
hold live. `Release` is that same two-sided unwind, the one frit runs
itself when a handoff fails.

## Releasing a claim

`Release` drops a claim: the local ref, and its copy on the remote. It is
the unwind for a claim that was minted but could not be stood up — a
worktree or agent that failed to come up behind it. A half-built lane
should not read as an abandoned hold. Release is best-effort on the local
side. It reports the remote delete, which is the one that matters.

## The marker records the lease

The marker commit's message records what the claim is for. The ref alone
then tells the full story, with no side channel. It carries the lane it
holds, the host that took it, the base it was dated against, and the plan
file it belongs to.

```text
plan 7: claim atlas-shader-unit

lane:     /home/you/git/atlas-shader-unit
host:     workshop
base:     4b825dc642cb6eb9a060e54bf8d69288fbee4904
plan:     plan/7_shader-unit.md
```
