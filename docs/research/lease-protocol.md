# The lease protocol: dead-agent detection over a git remote

2026-08-21. The design record behind plan 2608202144. Method: a
ten-finding adversarial code review of the previous claim design,
then four blind agents, none seeing the others' work — a scenario
enumerator (S1..S75), an independent protocol designer, a safety
attacker (A1..A8) and a liveness attacker (F1..F12). Everything below
survived, or was reshaped by, those reports. The plan carries the
implementable contract; this note carries the full enumeration and
the reasoning.

## The problem

frit coordinates autonomous agents across machines with exactly one
shared durable store: the git remote. Agents die at any instruction,
hosts lose power, networks partition, processes suspend for weeks and
resume (zombies), and no two machines share a clock. The previous
design was safe against races but not against death: a dead holder
blocked its plan until a human released it, holdership was inferred
from branch ancestry and status emoji, and the hold ref embedded a
slug computed from possibly-stale local state.

Requirements set by the operator: detect dead agents; heal without
human or agent intervention; enumerate every scenario with its
mitigation; every verb behaves predictably and testably in every
reachable state.

## Three insights that shaped the protocol

**One ref, not two.** A separate claims ref leaves a check-then-act
gap between verifying the lease and pushing work (safety attack A4:
not closable within the substrate, since refs are non-transactional).
When the work ref *is* the lease, the takeover CAS and the holder's
push contend on one ref the server serializes. There is no window
between revoked and fenced for anything that rides the ref.

**Healing must be passive.** A takeover path an agent must choose to
run never runs: `pick` hides held plans (F1), and observation state
that lives in a session dies with it (F2), so the staleness clock
never completes. Observation is therefore a side effect of every
fleet-reading verb, persisted per host, and `pick` itself surfaces
matured takeovers as candidates.

**Landed is observable; dead is only inferable.** A merged plan needs
no staleness window — the evidence is on origin, so scavenge fires
immediately and idempotently. Only death needs the window, and a
too-short window can never corrupt state (every transition is CAS),
only waste effort. T is tuned for economics, never for safety.

## The protocol

One ref per plan: `refs/heads/plan/<id>` — id only, no slug (S27,
S50, S51: renames and slug collisions cannot fork the arbitration
key). The ref carries the claim marker, work commits, beats, release
and takeover markers in one chain. The lease token is the tip SHA.

```text
plan <id>: claim | beat | release | takeover
epoch:   <E>            increases per acquisition, never per renewal
nonce:   <random>       fresh per commit: no two markers share a SHA
holder:  <machine-id>   stable id; hostname recorded as decoration
lane:    <path>         absolute worktree path on the holder
session: <herdr id>     the pane the lease is bound to; "-" if none
base:    <sha>          claim marker only, the freshly fetched base
```

The nonce is load-bearing (A3): SHA-based CAS is ABA-proof only if no
two commits can ever hash alike. Deterministic marker content would
let a delete-and-recreate reuse a SHA that a pending takeover or
release still expects, firing a stale CAS against a fresh lease.

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
| scavenge   | any tip proven landed   | ref deleted                          |

Consequences that do the safety work:

- A takeover marker is a **child of the old tip**. The zombie's local
  history becomes a sibling: its CAS push fails on a moved tip, and
  even a plain push is rejected as non-fast-forward (S16, S17, S30).
  The taker's lane checks out the takeover marker, inheriting every
  pushed commit; the ref is append-only until deletion — no
  transition force-pushes it backward or rebuilds it from base (A5).
- The ref is deleted only when its work is landed, so an unwind never
  deletes work (S9, S25): a failed handoff pushes a release marker
  instead and reports what it stood up (S73, S47).
- Losing a race, in any transition, means one failed CAS and a
  re-read. No transition retries blindly.

### Staleness

Staleness is observed change, dated on one clock. An observer records
`(ref, tip, first-seen, last-seen, samples)` in a per-host state
file. Stale means: the tip unchanged for longer than T of the
observer's own elapsed time, with at least two samples and no gap
over S_max between them. A voided window — the observer slept, or
origin was unreachable — restarts, so an origin outage resets every
observer instead of triggering a mass takeover on recovery (S23, and
the F5 amplifier). Lost observer state only ever delays takeover:
absent state reads as "first seen now", so amnesia is safe by
construction (F2 is answered by persistence, its loss by this rule).
Marker timestamps are never compared across machines (S33–S36).

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
server-serialized ref. The non-guarantee, stated plainly: raw
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
observation window like any other claimant. The fleet of one heals at
crash speed, and the landed-push lockout heals itself.

### Scavenge

Landed is evidence on origin, and the evidence must be fresh against
the tip it deletes (A2). Two classes:

- **Ancestry evidence is tip-coupled**: the observed tip is itself an
  ancestor of the default branch, and the CAS expects exactly that
  tip. A holder that has since renewed moved the tip, so the delete
  fails honestly.
- **Glyph and plan-gone evidence** (✅ on the default branch, or no
  plan file there) is decoupled from the tip, so it additionally
  requires a matured staleness window on the lease. A live, renewing
  holder can never be scavenged, and the reopen race — stale ✅ read
  at t, fresh lease minted at t+1 — collapses (A2, A6).

A ref carrying unlanded work commits is parked to
`refs/frit/rescue/<id>/<machine-id>` before deletion, so scavenge
never destroys work. Retries are idempotent CAS.

### Yield

A fenced lane runs `frit yield` (F4, F5): push local divergence to
the rescue ref (create-only, no lease needed — it is not a hold),
tear the lane down via herdr, exit clean. `next` and `show` list
rescue refs for a plan so stranded commits re-enter the flow.

### Parameters

Parameters live in `.frit.yml`, a per-repository convention like
`holds:` (F12): renewal period R, takeover window T, sample-gap bound
S_max, and takeover backoff — the k-th takeover of one epoch chain
waits k·T, damping ping-pong between two live-but-quiet agents (F3).
Choosing T is economics, not safety: a CAS takeover of a live holder
wastes effort but corrupts nothing, and the wasted work is bounded by
the rescue ref. T must dominate the longest legitimate quiet stretch;
R bounds the loss window for never-pushed work.

## The scenario matrix

Mechanism key: CAS (the transition table), FENCE (same-ref fencing),
OBS (staleness window), TAKE (takeover), SCAV (scavenge), RESUME
(self-resume), YIELD (rescue), VETO (herdr precedence), ID (id-only
ref, machine-id decoration), PARK (rescue before delete), TRUST
(outside the trust domain: reported by `orphans`/`board`, never
auto-mutated).

### Process death, at every lifecycle step

| #   | Scenario                                 | Outcome and mechanism                                     |
| --- | ---------------------------------------- | --------------------------------------------------------- |
| S1  | killed before local ref write            | nothing shared happened; retry is clean (CAS)             |
| S2  | killed after local write, before push    | origin never saw it; another claim wins honestly (CAS)    |
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

S11 is the one real loss, bounded by R: what was never pushed dies
with the host. R is the price of durability, and beats make the gap
explicit.

### Host death, suspension, zombies

| #   | Scenario                              | Outcome and mechanism                                              |
| --- | ------------------------------------- | ------------------------------------------------------------------ |
| S14 | power loss mid-push                   | as S3; local repo damage is that host's own concern                |
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
| S22 | observer partitioned             | display carries observed-at age; advice never mutates (OBS)       |
| S23 | everyone partitioned, origin up  | all windows void on heal; no mass takeover (OBS S_max)            |
| S24 | asymmetric: push ok, fetch fails | classification degrades to unknown and says so; CAS still decides |
| S25 | stale unwind delete after heal   | no unleased deletes exist; release is CAS on own tip              |

### Races

| #   | Scenario                             | Outcome and mechanism                                       |
| --- | ------------------------------------ | ----------------------------------------------------------- |
| S26 | N claimants, one plan                | one CAS winner; losers read the marker and report (CAS)     |
| S27 | rename between two claimants         | ID: the ref has no slug; one ref, one winner                |
| S28 | human deletes ref mid-claim          | TRUST; the retry claims a free ref honestly                 |
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

| #   | Scenario                        | Outcome and mechanism                                         |
| --- | ------------------------------- | ------------------------------------------------------------- |
| S37 | claim ref hand-deleted          | holder's next CAS fails → refuses, reports (FENCE, TRUST)     |
| S38 | claim ref hand-force-pushed     | as S37; forged markers are TRUST                              |
| S39 | work ref force-pushed backward  | ABA on a stale takeover CAS; cooperative model, TRUST         |
| S40 | remote GC reaps deleted markers | forensics degrade; rescue refs keep the work (PARK)           |
| S41 | remote rewritten or migrated    | every CAS fails safe; fleet re-acquires; TRUST                |
| S42 | two remotes, split coordination | unsupported: one coordination remote, declared in `.frit.yml` |
| S43 | origin URL edited mid-lifecycle | observer state keys on remote URL; old windows void (OBS)     |
| S44 | fork-based flow                 | unsupported, documented; coordination is the shared remote    |
| S71 | origin restored from backup     | holders' CAS fail → refuse and re-acquire; converges (FENCE)  |

### Identity anomalies

| #   | Scenario                          | Outcome and mechanism                                             |
| --- | --------------------------------- | ----------------------------------------------------------------- |
| S45 | two agents, one plan, one host    | one lease, one bound session; the other's verbs refuse (VETO)     |
| S46 | worktree path reused              | the marker binds plan id, machine and path; mismatch refuses (ID) |
| S47 | worktree debris fails the handoff | release marker + the error names the path (CAS)                   |
| S48 | hostname changes                  | identity is machine-id; hostname is decoration (ID)               |
| S49 | hostname collides                 | as S48; token fencing serializes even cloned machine-ids (A1)     |
| S66 | NFS-shared clone across hosts     | unsupported, documented: a lane is one host's path                |

### Lifecycle anomalies

| #   | Scenario                        | Outcome and mechanism                                         |
| --- | ------------------------------- | ------------------------------------------------------------- |
| S50 | plan file renamed after claim   | ID: ref name never contained the slug                         |
| S51 | slug collision across plans     | ID: no slugs in refs                                          |
| S52 | plan deleted while claimed      | SCAV on plan-gone evidence after a window; PARK first         |
| S53 | plan id reused                  | forbidden by proto (minute ids); scavenge old ref by evidence |
| S54 | squash-merge, status never ✅   | SCAV accepts landed evidence, not only the glyph              |
| S55 | merge + branch auto-delete      | ref gone is released; a 🔳 unheld plan is claimable           |
| S56 | local branch deleted by hand    | origin is the only authority verbs consult (CAS)              |
| S57 | plan re-opened after done       | old ref scavenged if landed; fresh acquire (SCAV, CAS)        |
| S58 | released before the PR merges   | window of duplicate claim; human process, TRUST               |
| S59 | status flipped ✅ early by hand | dependents unblock on a lie; TRUST, `doctor`'s concern        |
| S70 | claim dated against an old base | acquire fetches the base at claim time (CAS)                  |
| S75 | default branch renamed          | evidence follows origin's HEAD, refreshed per read            |

### Cross-layer: herdr and frit disagree

| #   | Scenario                         | Outcome and mechanism                                                      |
| --- | -------------------------------- | -------------------------------------------------------------------------- |
| S60 | herdr down at claim time         | lease valid, lane pending; RESUME stands it up later                       |
| S61 | herdr down at observation        | no veto either way; OBS window governs (VETO)                              |
| S62 | host unreachable, agents pushing | tip advances → observations reset; no takeover (OBS)                       |
| S63 | pane alive, lease released       | agent's next CAS fails → fenced → YIELD                                    |
| S64 | branch repurposed by hand        | verbs check branch ↔ plan id ↔ marker; mismatch refuses (ID)               |
| S65 | herdr restarts, loses panes      | renewals continue via the agent's own verbs; veto lapses to OBS            |
| S72 | claim and start race on one host | one winner; the loser's refusal names the winning lane                     |
| S73 | prompt fails after agent start   | release marker, agent fenced at its first verb, pane reported (CAS, FENCE) |
| S74 | same plan id in two repos        | lanes key host:repo:id; pane names carry the repo                          |

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
| F8  | chain grows without bound          | beats squash away at merge; renewal rate-limited to R                         |
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

## Residual risks, stated plainly

- Raw git against origin — force-pushes, hand deletes, forged
  markers, history rewrites — is inside the trust domain of write
  access and outside the fence. frit reports what it sees; it never
  fights a human for a ref (S37–S44, S69). A forge-side branch
  protection rule on `plan/*` would shrink this and is recommended,
  never required.
- Work never pushed dies with its host (S11). Bounded by R.
- External side effects a taken-over holder already fired are not
  un-fired; the epoch is exported for resources that honor fencing
  tokens, and nothing else can use it.
- The irreducible window (A4): mutations that do not ride the lane
  ref — an external side effect, or the PR merge to the default
  branch itself — happen after a fence check that cannot be atomic
  with them. A suspension spanning exactly that gap yields a
  duplicated effect. The fence keeps every ref mutation safe; for the
  rest, the window is accepted, shrunk by the veto and backoff, and
  loud when it happens.
- A forward clock step inside a valid window can fire an early
  takeover. Safe, wasteful, damped by backoff (S35).
- Healing needs ambient verb activity somewhere in the fleet. A fully
  idle fleet reaps nothing — and wants nothing.

## Rejected alternatives

- **A separate claims namespace** (`refs/frit/claims/<id>`): the
  first redesign draft. Killed by A4 — the check-then-act gap between
  the claims ref and the work branch is not closable when refs are
  non-transactional. The single-ref design removes the gap instead of
  narrowing it.
- **TTL leases on wall clocks**: revokes a slow-but-alive agent on
  the strength of clocks no two machines share. Staleness here is
  observed non-change on one clock, and even then a wrong call is
  CAS-safe.
- **Identity-based self-recognition** (host, or host+lane strings):
  admits cloned machines and reused paths as the same holder with no
  race needed (A1). The token is the identity.
- **Renewal keyed to activity in the lane directory**: bystanders
  renew a corpse's lease forever (F10). Renewal requires the bound
  session.
- **A daemon**: would make observation continuous, but frit has no
  daemon and herdr already fills the per-host liveness role; piggyback
  observation plus persisted windows reaches the same healing with
  verbs alone.
