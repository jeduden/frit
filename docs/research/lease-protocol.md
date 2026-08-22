# The lease protocol: dead-agent detection over a git remote

2026-08-21. The design record behind plan 2608202144. The method was
a ten-finding adversarial code review of the previous claim design,
followed by four blind agents that did not see each other's work: a
scenario enumerator (S1..S75), an independent protocol designer, a
safety attacker (A1..A8) and a liveness attacker (F1..F12). The plan
holds the implementable contract; this note holds the full
enumeration and the reasoning behind it.

## The problem

frit coordinates autonomous agents across machines with exactly one
shared durable store: the git remote. Agents die at any instruction,
hosts lose power, networks partition, processes suspend for weeks and
resume (zombies), and no two machines share a clock. The previous
design was safe against races but not against death: a dead holder
blocked its plan until a human released it, holdership was inferred
from branch ancestry and status emoji, and the hold ref embedded a
slug computed from possibly-stale local state.

The requirements: detect dead agents; recover without human or agent
intervention; enumerate every scenario with its mitigation; make
every verb behave predictably and testably in every reachable state.

## What shaped the protocol

**One ref, not two.** A separate claims ref leaves a check-then-act
gap between verifying the lease and pushing work (safety attack A4:
not closable within the substrate, since refs are non-transactional).
When the work ref is also the lease, the takeover CAS and the
holder's push contend on one ref that the server serializes. For
anything that goes through the ref, there is no window between
losing the lease and being fenced.

**Healing has to be passive.** A takeover path that an agent has to
choose to run in practice never runs: `pick` hides held plans (F1),
and observation state kept in a session dies with it (F2), so the
staleness clock never completes. Observation therefore happens as a
side effect of every fleet-reading verb, is persisted per host, and
`pick` itself lists matured takeovers as candidates.

**Merged work can be read; a dead agent can only be inferred.**
Whether a plan's work has merged is a fact on origin, so scavenge
acts on it immediately and idempotently, with no staleness window.
Only death needs the window, and a window that is too short cannot
corrupt state, because every transition is a CAS; it only wastes
effort. T is therefore chosen for cost, not for correctness.

## Terms

- **The work ref**: `refs/heads/plan/<id>`, the one ref per plan
  that is both the claim and the branch the work rides on. Older
  docs say "claim branch", "work branch" or "hold ref"; in the new
  design those all name this one ref, and this note calls it the
  work ref throughout.
- **Token**: the work ref's tip SHA as a holder last pushed it. The
  holder's copy persists in the lane's git dir.
- **Lane**: one worktree on one host, working one plan. Identified
  by (machine-id, absolute worktree path).
- **Machine-id**: the host's stable identifier (`/etc/machine-id` on
  Linux); hostnames rename and collide, so they are display only.
- **Verb**: a frit subcommand. There is no daemon; every protocol
  action happens inside some verb run.
- **Beat**: an empty commit the holder's session pushes to renew the
  lease when it has no work to push. Minted at most once per R.
- **Epoch**: a counter in the marker trailers, incremented by each
  acquisition (acquire, re-acquire, takeover), never by renewal.
- **k**: the number of takeover markers already in the ref's chain.
  Read from the chain itself, so every observer computes the same k,
  and it resets when the ref is deleted.
- **Landed**: the plan's work has reached origin's default branch.
- **Matured window**: a staleness observation that satisfies the
  rule in the Staleness section.
- **herdr**: the per-host daemon that owns panes, worktrees and
  prompts; frit reads it for session liveness.
- **Forge**: the git hosting service (GitHub here), whose merge and
  branch-delete behavior frit observes but does not control.

## The protocol

One ref per plan: the work ref, `refs/heads/plan/<id>` — id only, no
slug (S27, S50, S51: renames and slug collisions cannot fork the
arbitration key). It carries the claim marker, work commits, beats,
release and takeover markers in one chain. The lease token is the
tip SHA.

```text
plan <id>: claim | beat | release | takeover
epoch:   <E>            increases per acquisition, never per renewal
nonce:   <random>       fresh per commit: no two markers share a SHA
holder:  <machine-id>   stable id; hostname recorded as decoration
lane:    <path>         absolute worktree path on the holder
session: <herdr id>     the pane the lease is bound to; "-" if none
base:    <sha>          claim marker only, the freshly fetched base
```

The nonce is required for correctness (A3). SHA-based CAS is only
ABA-proof if no two commits can hash alike. With deterministic marker
content, a delete followed by a recreate could reuse a SHA that a
pending takeover or release still expects, and that stale CAS would
then fire against a fresh lease.

Every transition is one server-side CAS (`--force-with-lease` with an
exact expected value). The server is the arbiter; frit never decides
holdership from a local view.

| Transition | Expected old value      | New tip                              |
| ---------- | ----------------------- | ------------------------------------ |
| acquire    | ref absent              | claim marker, epoch 1, on fresh base |
| re-acquire | release marker tip      | claim marker, epoch E+1, child of it |
| renew      | holder's own last tip   | work commit(s) or beat, same epoch   |
| release    | holder's own last tip   | release marker, same epoch           |
| takeover   | the observed stale tip  | takeover marker, epoch E+1, child    |
| complete   | own tip, landed on main | ref deleted                          |
| scavenge   | the observed tip*       | ref deleted; unlanded work parked    |

\* Scavenge needs fresh evidence, not landed-proof alone; the
Scavenge section defines the three evidence classes and when work is
parked first.

Consequences:

- A takeover marker is a **child of the old tip**. The zombie's local
  history becomes a sibling: its CAS push fails on a moved tip, and
  even a plain push is rejected as non-fast-forward (S16, S17, S30).
  The taker's lane checks out the takeover marker, inheriting every
  pushed commit; the ref is append-only until deletion — no
  transition force-pushes it backward or rebuilds it from base (A5).
- The ref is deleted only by complete or scavenge, and anything
  unlanded on it is parked to a rescue ref before the delete, so no
  deletion loses work (S9, S25). An unwind never deletes: a failed
  handoff pushes a release marker instead and reports what it stood
  up (S73, S47).
- Losing a race, in any transition, means one failed CAS and a
  re-read. No transition retries blindly.

### Staleness

Staleness is observed change, dated on one clock. An observer records
`(ref, tip, first-seen, last-seen, samples)` in a per-host state
file, adding a sample whenever a fleet-reading verb fetches the tip.
Stale means: the samples show one unchanged tip, they span more than
T of the observer's own elapsed time, and no gap between consecutive
samples exceeds S_max. S_max must be well below T — the defaults set
S_max = T/4, so a matured window holds at least five samples. A
voided window — a gap over S_max, because the observer slept or
origin was unreachable — restarts, so an origin outage resets every
observer instead of triggering a mass takeover on recovery (S23, and
the F5 amplifier). Lost observer state only ever delays a takeover:
absent state reads as "first seen now", so losing the file is safe
(persistence answers F2; this rule covers losing it anyway). Marker
timestamps are never compared across machines (S33–S36).

### Fencing

Fencing is the CAS itself, and the token is the tip SHA — never
identity strings. The holder is whoever minted the current tip. The
`holder:` and `lane:` trailers are for reporting, not for passing any
check: A1 showed that string-identity fencing admits two holders
deterministically — cloned machine-ids, or a reused lane path,
produce equal strings with no race needed. The token persists in the
lane's git dir, so it survives the process. Any verb in the lane
pushes with the lane's recorded tip as expected value; if the lease
moved, the push fails and the verb refuses, naming the new holder.
On a failed renewal the verb re-reads first and refuses only if the
new tip is foreign — a same-lane race just continues from the newer
tip (A7).

The guarantee: no verb-mediated mutation lands after a takeover,
with no window, because revocation and mutation contend on one
server-serialized ref. What it does not guarantee: raw
`git push --force`, pushes to other refs, and external side effects
are outside the fence (S37–S39, S68, S69, A8). Write access to
origin is the trust domain.

### Liveness precedence

Explicit and testable (S61–S63, F10):

1. A live herdr session bound to the lease vetoes takeover and renews
   on the holder's behalf — but only a positive answer counts; an
   unreachable host or a dead daemon is no veto (F3).
2. Failing a veto, the staleness window governs.
3. A fenced-out session's next verb refuses and offers `yield`.

Renewal is bound to the recorded live session, never to "a verb ran
in this directory" — F10 showed bystander activity (an operator
inspecting a dead lane, a cron job) would otherwise renew a corpse's
lease forever. Read-only verbs never renew.

### Self-resume

A lane whose persisted token matches origin's current tip, when local
herdr confirms no live session owns the lane, resumes immediately —
no window (F9, F11, S3, S21). The proof of ownership is the token on
disk, not the identity strings (A1): a cloned machine or reused path
without the lane's recorded tip gets no shortcut, and two clones that
both carry it serialize on the next CAS — one continues, the other is
fenced. A lane that lost its local state falls back to the
observation window like any other claimant. A fleet of one recovers
as soon as it restarts. The same path covers a push that landed
while the client saw an error (S3): the lane's stored token matches
the tip origin kept, so the next run resumes instead of being locked
out of its own lease.

### Scavenge

The evidence must be fresh against the tip it deletes (A2). Three
classes — the first two landed, the third added 2026-08-22 by plan
2608212218's `reap` verb:

- **Ancestry evidence is tied to the tip**: the observed tip is
  itself an ancestor of the default branch, and the CAS expects
  exactly that tip. A holder that has since renewed moved the tip,
  so the delete fails.
- **Glyph and plan-gone evidence** (✅ on the default branch, or no
  plan file there) is not tied to the tip, so it additionally
  requires a matured staleness window on the lease. A live, renewing
  holder can never be scavenged, and the reopen race (a stale ✅
  read, then a fresh lease minted a moment later) is closed (A2,
  A6).
- **Abandonment evidence** carries no landed proof at all: the lease
  is observed stale, or its bound session is one herdr positively
  confirms dead. It is takeover-grade evidence pointed at deletion
  instead of seizure — the plan returns to startable rather than
  changing hands — and the CAS on the observed tip still fences a
  holder that renewed. "No local checkout" is deliberately not in
  this class: the checkout may be another machine's, so `reap`
  refuses a live lease however unstaffed it looks locally.

A ref carrying unlanded work commits is parked to
`refs/frit/rescue/<id>/<machine-id>` before deletion, so scavenge
never destroys work. Retries are idempotent CAS.

The park half stands alone as `ParkUnlanded`, for a teardown that
deletes a local branch through git porcelain rather than the work
ref through a CAS. `reap`'s stranded teardown parks the branch tip
before `git branch -D`: glyph evidence is not tied to that tip, and
a follow-up commit the squash never carried would otherwise be
destroyed. A park that cannot happen refuses the delete.

Classifying a ref as gone takes a remote's positive answer. An
`ls-remote` that fails is surfaced as a fault, never folded into
"already deleted", so an unreachable origin cannot make a scavenge
clean the local ref and report a no-op that never happened.

### Yield

A fenced lane runs `frit yield` (F4, F5): push local divergence to
the rescue ref (create-only, no lease needed, since it is not a
hold), tear the lane down via herdr, and exit. `next` and `show`
list rescue refs for a plan, so stranded commits are found again.

### Parameters

Parameters live in `.frit.yml`, a per-repository convention like
`holds:` (F12): renewal period R, takeover window T, sample-gap bound
S_max, and takeover backoff — a takeover waits k·T instead of T,
where k counts the takeover markers already in the ref's chain, so
every observer computes the same k and oscillation between two live
but quiet agents damps out (F3). T affects cost, not correctness: a
takeover of a live holder wastes effort but corrupts nothing, and
the rescue ref bounds the wasted work. T should exceed the longest
legitimate quiet stretch; R bounds how much never-pushed work a dead
host can take with it.

## The scenario matrix

Mechanism key: CAS (the transition table), FENCE (same-ref fencing),
OBS (staleness window), TAKE (takeover), SCAV (scavenge), RESUME
(self-resume), YIELD (rescue), VETO (herdr precedence), ID (id-only
ref, machine-id identity), PARK (rescue before delete), TRUST (an
actor inside the trust domain — anyone with write access to origin;
frit reports what it sees via `orphans` and `board` and does not
defend against them).

### Process death, at every lifecycle step

| #   | Scenario                                 | Outcome and mechanism                                     |
| --- | ---------------------------------------- | --------------------------------------------------------- |
| S1  | killed before local ref write            | nothing shared happened; retry is clean (CAS)             |
| S2  | killed after local write, before push    | origin never saw it; another claim can win (CAS)          |
| S3  | killed mid-push, server committed        | RESUME: same lane reclaims instantly; else OBS→TAKE       |
| S4  | killed before worktree creation          | RESUME on the same host; elsewhere OBS→TAKE               |
| S5  | killed between worktree and agent start  | as S4; board shows held lane, no session                  |
| S6  | killed between agent start and prompt    | agent idles; no renewals → OBS→TAKE; VETO cannot fire     |
| S7  | observer saw a claim that then unwound   | tip changed or ref gone → observation resets (OBS)        |
| S8  | unwind's remote delete fails             | no deletes on unwind: release marker instead (CAS)        |
| S9  | unwind deletes a branch with pushed work | impossible: only landed refs are deleted (CAS, PARK)      |
| S10 | killed mid-phase, work pushed            | TAKE inherits pushed commits (takeover is a child)        |
| S11 | killed mid-phase, work only local        | TAKE proceeds from last push; local-only work is the loss |
| S12 | killed after merge, before status flip   | SCAV on landed evidence, no window needed                 |
| S13 | status flipped on branch, not merged     | evidence reads origin's default branch only (SCAV)        |

S11 is the one real loss, and R bounds it: what was never pushed
dies with the host.

### Host death, suspension, zombies

| #   | Scenario                              | Outcome and mechanism                                              |
| --- | ------------------------------------- | ------------------------------------------------------------------ |
| S14 | power loss mid-push                   | as S3; any local repo damage stays local                           |
| S15 | host dies holding a claim, never back | OBS→TAKE after T; no human needed                                  |
| S16 | host resurrected days later           | FENCE: sibling history, every push rejected; YIELD                 |
| S17 | suspended weeks, plan re-claimed      | FENCE as S16; divergence parked by YIELD                           |
| S18 | zombie re-runs its own claim          | RESUME only when no live session owns the lane; else refuse (VETO) |
| S19 | zombie pushes to a completed plan     | verb CAS fails (ref absent ≠ own tip); raw push is TRUST           |

### Partitions

| #   | Scenario                         | Outcome and mechanism                                             |
| --- | -------------------------------- | ----------------------------------------------------------------- |
| S20 | worker partitioned mid-work      | renewals fail → holder self-fences fast; OBS→TAKE; YIELD on heal  |
| S21 | push landed during partition     | RESUME recognizes the lane's own token on heal                    |
| S22 | observer partitioned             | display carries the observed-at age; nothing mutates (OBS)        |
| S23 | everyone partitioned, origin up  | all windows void on heal; no mass takeover (OBS S_max)            |
| S24 | asymmetric: push ok, fetch fails | classification degrades to unknown and says so; CAS still decides |
| S25 | stale unwind delete after heal   | no unleased deletes exist; release is CAS on own tip              |

### Races

| #   | Scenario                             | Outcome and mechanism                                       |
| --- | ------------------------------------ | ----------------------------------------------------------- |
| S26 | N claimants, one plan                | one CAS winner; losers read the marker and report (CAS)     |
| S27 | rename between two claimants         | ID: the ref has no slug; one ref, one winner                |
| S28 | human deletes ref mid-claim          | TRUST; the retry claims the now-free ref                    |
| S29 | release races a loser's read         | loser reports unknown holder, retries; CAS decides          |
| S30 | zombie vs new claimant on one branch | FENCE: sibling history, non-fast-forward                    |
| S31 | orphan report vs sleeping host       | report only; TAKE waits for OBS window; VETO if host wakes  |
| S32 | two same-host sessions race          | one CAS winner; loser's refusal names the winning lane (ID) |

### Clocks

| #   | Scenario                | Outcome and mechanism                                                   |
| --- | ----------------------- | ----------------------------------------------------------------------- |
| S33 | frozen clock on worker  | timestamps are decoration; liveness is tip change (OBS)                 |
| S34 | clock steps backward    | same; commit dates mislead humans only                                  |
| S35 | clock steps far forward | observer window may fire early; CAS makes it safe, backoff damps (TAKE) |
| S36 | cross-host clock skew   | no cross-machine timestamp is ever compared (OBS)                       |

### Storage anomalies

| #   | Scenario                                | Outcome and mechanism                                                                                                                                                         |
| --- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| S37 | work ref hand-deleted                   | holder's next CAS fails → refuses, reports (FENCE, TRUST)                                                                                                                     |
| S38 | work ref hand-force-pushed              | as S37; forged markers are TRUST                                                                                                                                              |
| S39 | work ref force-pushed backward          | ABA on a stale takeover CAS; cooperative model, TRUST                                                                                                                         |
| S40 | remote GC reaps deleted markers         | marker history is lost; rescue refs keep the work (PARK)                                                                                                                      |
| S41 | remote rewritten or migrated            | every CAS fails safe; fleet re-acquires; TRUST                                                                                                                                |
| S42 | two remotes, split coordination         | unsupported: one coordination remote, declared in `.frit.yml`                                                                                                                 |
| S43 | origin URL edited mid-lifecycle         | observer state keys on remote URL; old windows void (OBS)                                                                                                                     |
| S44 | fork-based flow                         | unsupported, documented; coordination is the shared remote                                                                                                                    |
| S67 | `fetch --prune` races a read            | one ls-remote snapshot per decision; a failed CAS re-reads (CAS)                                                                                                              |
| S68 | default branch force-pushed             | ancestry evidence stops matching; glyph evidence remains; TRUST                                                                                                               |
| S69 | marker body forged                      | trailers are reporting only; the token is the fence (FENCE, TRUST)                                                                                                            |
| S71 | origin restored from backup             | holders' CAS fail → refuse and re-acquire; converges (FENCE)                                                                                                                  |
| S78 | two parks from one lane, different tips | the create-only rescue ref collides on the second park; each tip gets its own content-addressed ref, so parks never conflict, and `orphans` lists leftover rescue refs (PARK) |

### Identity anomalies

| #   | Scenario                          | Outcome and mechanism                                                                |
| --- | --------------------------------- | ------------------------------------------------------------------------------------ |
| S45 | two agents, one plan, one host    | one lease, one bound session; the other's verbs refuse (VETO)                        |
| S46 | worktree path reused              | the reused lane holds no matching token, so it is a claimant, not the holder (FENCE) |
| S47 | worktree debris fails the handoff | release marker + the error names the path (CAS)                                      |
| S48 | hostname changes                  | identity is machine-id; hostname is decoration (ID)                                  |
| S49 | hostname collides                 | as S48; token fencing serializes even cloned machine-ids (A1)                        |
| S66 | NFS-shared clone across hosts     | unsupported, documented: a lane is one host's path                                   |

### Lifecycle anomalies

| #   | Scenario                                                                             | Outcome and mechanism                                                                                                                                                                           |
| --- | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| S50 | plan file renamed after claim                                                        | ID: ref name never contained the slug                                                                                                                                                           |
| S51 | slug collision across plans                                                          | ID: no slugs in refs                                                                                                                                                                            |
| S52 | plan deleted while claimed                                                           | SCAV on plan-gone evidence after a window; PARK first                                                                                                                                           |
| S53 | plan id reused                                                                       | forbidden by proto (minute ids); scavenge old ref by evidence                                                                                                                                   |
| S54 | squash-merge, status never ✅                                                        | SCAV accepts landed evidence, not only the glyph                                                                                                                                                |
| S55 | merge + branch auto-delete                                                           | ref gone is released; a 🔳 unheld plan is claimable                                                                                                                                             |
| S56 | local branch deleted by hand                                                         | origin is the only authority verbs consult (CAS)                                                                                                                                                |
| S57 | plan re-opened after done                                                            | old ref scavenged if landed; fresh acquire (SCAV, CAS)                                                                                                                                          |
| S58 | released before the PR merges                                                        | window of duplicate claim; human process, TRUST                                                                                                                                                 |
| S59 | status flipped ✅ early by hand                                                      | dependents unblock too early; TRUST, `doctor`'s concern                                                                                                                                         |
| S70 | claim dated against an old base                                                      | acquire fetches the base at claim time (CAS)                                                                                                                                                    |
| S75 | default branch renamed                                                               | evidence follows origin's HEAD, refreshed per read                                                                                                                                              |
| S79 | scavenge/mint delete a branch a worktree still stands on                             | a fresh worktree-list read gates every local `update-ref -d`; a branch still checked out keeps its local copy, only the remote delete and park proceed (SCAV)                                   |
| S80 | local default branch lags its own fetched remote-tracking ref                        | `Gather` compares the two and reports it as a problem; discovery never fetches on its own, so this is the one signal a stale read gets                                                          |
| S81 | unstaffed hold, holder alive on another machine                                      | `reap` refuses: a drop needs abandonment evidence — a matured window or a dead session — never the missing local checkout (OBS, SCAV, A2)                                                       |
| S82 | reaped squash-landed branch carries a follow-up commit                               | the branch tip is parked to the rescue ref before `branch -D`; a park that cannot happen refuses the whole teardown (PARK)                                                                      |
| S83 | origin unreadable while scavenge classifies the ref                                  | surfaced as a fault, local ref kept; "gone" is only ever a remote's positive answer (SCAV)                                                                                                      |
| S84 | local default branch normally lags origin, so it is never authoritative for evidence | landed evidence — status glyph and `--merged` ancestry — reads origin's default branch via its remote-tracking ref, never a local `main` that normally trails (SCAV; the invariant of S13, S75) |
| S85 | `origin/HEAD` unset, so `DefaultRef` falls back to a local default branch            | `DefaultRef` reaches `refs/remotes/origin/<default>` before any local `main`, so a squash-landed `✅` is seen however far `main` lags (SCAV; see S54, S80, S84)                                 |

### Cross-layer: herdr and frit disagree

| #   | Scenario                            | Outcome and mechanism                                                                                                                                         |
| --- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| S60 | herdr down at claim time            | lease valid, lane pending; RESUME stands it up later                                                                                                          |
| S61 | herdr down at observation           | no veto either way; OBS window governs (VETO)                                                                                                                 |
| S62 | host unreachable, agents pushing    | tip advances → observations reset; no takeover (OBS)                                                                                                          |
| S63 | pane alive, lease released          | agent's next CAS fails → fenced → YIELD                                                                                                                       |
| S64 | branch repurposed by hand           | the lane's token no longer matches the tip; verbs refuse to act as holder (FENCE)                                                                             |
| S65 | herdr restarts, loses panes         | renewals continue via the agent's own verbs; veto lapses to OBS                                                                                               |
| S72 | claim and start race on one host    | one winner; the loser's refusal names the winning lane                                                                                                        |
| S73 | prompt fails after agent start      | release marker, agent fenced at its first verb, pane reported (CAS, FENCE)                                                                                    |
| S74 | same plan id in two repos           | lanes key host:repo:id; pane names carry the repo                                                                                                             |
| S76 | pane gone before the window matures | no live session, window not matured, token cannot self-resume: a silent dead end. `orphans` names the deserted hold from the veto, not the window (VETO, OBS) |
| S77 | deserted lane on its own host       | the dead host sees local commits ahead of origin; a verb rebuilds the pane in place, or yield parks the suffix when resume is declined (RESUME, YIELD)        |

### Liveness traps, from the blind liveness attack

| #   | Trap                               | Mechanism that closes it                                                      |
| --- | ---------------------------------- | ----------------------------------------------------------------------------- |
| F1  | healing hidden from the verb loop  | observation piggybacks on every fleet read; `pick` surfaces matured takeovers |
| F2  | observation dies with the session  | per-host persisted state; loss only delays (OBS)                              |
| F3  | takeover of the living, ping-pong  | VETO for reachable hosts; k·T backoff damps oscillation                       |
| F4  | fenced lane is a dead end          | YIELD: rescue ref, teardown, clean exit                                       |
| F5  | divergent work unmergeable         | takeover records its start tip; YIELD pushes the suffix                       |
| F6  | done-dominates never fires         | SCAV accepts landed evidence beyond the glyph                                 |
| F7  | immortal lease, plan gone          | SCAV on plan-nonexistence, PARK first                                         |
| F8  | chain grows without bound          | beats never land: the PR squashes them out; renewal rate-limited to R         |
| F9  | crash loop locked out of own lease | RESUME by the lane's persisted token, no window                               |
| F10 | bystander cwd activity renews      | renewal requires the bound live session                                       |
| F11 | fleet of one heals never           | RESUME is a first-class path in `pick` and `claim`                            |
| F12 | T is private per observer          | parameters live in `.frit.yml`, travel with the repo                          |

### Safety attacks, from the blind safety attack

| #   | Attack                                 | Answer in this design                                     |
| --- | -------------------------------------- | --------------------------------------------------------- |
| A1  | identity-string fencing admits clones  | fencing and resume are token-based; strings are reporting |
| A2  | stale ✅ read scavenges a live lease   | evidence freshness: tip-coupled ancestry, or window       |
| A3  | delete/recreate reuses a SHA (ABA)     | fresh nonce in every marker; no SHA ever repeats          |
| A4  | check-then-act across two refs         | one ref: revocation and mutation contend on the same CAS  |
| A5  | takeover rebuilds the branch from base | takeover checks out the old tip's child; append-only      |
| A6  | A2+A3 composed around a GC window      | collapses once A2 and A3 are closed                       |
| A7  | self-race read as eviction             | re-read on failed renewal; refuse only a foreign tip      |
| A8  | renewal and takeover identical on wire | in the trust domain; reported, not fought                 |

## Residual risks

- Raw git against origin — force-pushes, hand deletes, forged
  markers, history rewrites — is inside the trust domain of write
  access and outside the fence. frit reports what it sees; it does
  not contend with a person for a ref (S37–S44, S69). A forge-side
  branch protection rule on `plan/*` would shrink this and is
  recommended, never required.
- Work never pushed dies with its host (S11). R bounds the loss.
- External side effects a taken-over holder already fired are not
  un-fired; the epoch is exported for resources that honor fencing
  tokens, and nothing else can use it.
- The irreducible window (A4): mutations that do not ride the work
  ref — an external side effect, or the PR merge to the default
  branch itself — happen after a fence check that cannot be atomic
  with them. A suspension spanning exactly that gap duplicates the
  effect. The fence keeps every ref mutation safe; for the rest, the
  window is accepted, made smaller by the veto and backoff, and
  visible when it happens.
- A forward clock step inside a valid window can fire an early
  takeover. That is safe but wasteful, and backoff damps it (S35).
- Healing needs some verb activity somewhere in the fleet. A fully
  idle fleet reaps nothing; it also has nobody waiting for the plan.

## Rejected alternatives

- **A separate claims namespace** (`refs/frit/claims/<id>`): the
  first redesign draft. Dropped after A4: the check-then-act gap
  between the claims ref and the work branch cannot be closed when
  refs are non-transactional. The single-ref design removes the gap
  instead of narrowing it.
- **TTL leases on wall clocks**: this revokes a slow but alive agent
  on the strength of clocks no two machines share. Staleness here is
  observed non-change on one clock, and even then a wrong call is
  CAS-safe.
- **Identity-based self-recognition** (host, or host+lane strings):
  this admits cloned machines and reused paths as the same holder,
  with no race needed (A1). The token is the identity.
- **Renewal keyed to activity in the lane directory**: bystanders
  would keep renewing a dead agent's lease (F10). Renewal requires
  the bound session.
- **A daemon**: would make observation continuous, but frit has no
  daemon and herdr already fills the per-host liveness role; piggyback
  observation plus persisted windows reaches the same healing with
  verbs alone.
