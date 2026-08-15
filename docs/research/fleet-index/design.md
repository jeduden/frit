---
summary: >-
  The measured state of this machine, the cwd join that links an agent to
  a plan, reading plans out of git objects without a checkout, multi-host
  topology, and the dispatch ladder that grows a read-only board into a
  dispatcher without building a prompt UI.
---
# The fleet index

## What the ground truth looked like

Measured on one workstation, 2026-08-15. Repositories are anonymised;
these numbers set the scale the index has to handle and show where the
convention is consistent enough to rely on.

| Repo | Plans | Worktrees | Remote refs | Convention |
| ---- | ----- | --------- | ----------- | ---------- |
| A    | 152   | 80        | 313         | full       |
| B    | 171   | 7         | 899         | full       |
| C    | 0     | 3         | —           | partial    |
| D    | 0     | 1         | —           | none       |

Plan front matter is stable across both adopting repositories — `id`,
`title`, `status`, `summary`, `model`, `depends-on` — and 105 of A's
152 plans declare dependencies, so there is a real DAG to walk rather
than a flat list. Status is a four-value vocabulary already in use:
🔲 not started, 🔳 in progress, ✅ done, ⛔ superseded.

Two traps showed up immediately.

**Ids collide.** A allocates minute-precision timestamps
(`2608142306`); B allocates counters (`100`). A fleet-wide key must
be `host:repo:id`, never `id` alone.

**Adoption is uneven.** Two of four repositories have no `plan/`
directory at all, so the index has to degrade to a branch-and-agent view
rather than assuming the convention.

## The join is cwd, and it was already reliable

Every question the tool answers — *which agent is working on which task*
— resolves through one chain, and no step needs new bookkeeping:

```text
pane (herdr agent list)
  → cwd
  → worktree root (git rev-parse --show-toplevel)
  → branch
  → plan id embedded in plan/<id>-<slug>
  → plan file: title, status, dependencies
```

`herdr agent list` returns JSON per pane carrying `agent`,
`agent_status`, `cwd`, `foreground_cwd`, `pane_id`, `workspace_id`, the
agent session id, and the terminal title.

Two caveats worth designing around. `cwd` is the pane's shell directory
and can drift from the worktree root, so it must be resolved with
`git rev-parse --show-toplevel` rather than string-matched against a
worktree list. And `agent_status` reads `unknown` for a non-integrated
agent — three workspaces showed exactly that — so the board needs an
honest third state rather than a false idle.

## Reading branches without checking them out

The hardest requirement — indexing plans across *many branches* — has a
clean answer that avoids checking out or fetching working trees. Plan
files are read straight out of git's object store.

The walk costs a fixed number of processes per repository regardless of
ref count:

1. `for-each-ref` for every ref, local, remote-tracking and tags alike.
2. One `cat-file --batch-check` resolving `<ref>:plan` for every ref at
   once, giving the tree object per ref.
3. One `ls-tree -r` per *distinct* tree. Branches that share a plan
   directory share its tree object, so the distinct trees are far fewer
   than the refs.
4. One `cat-file --batch` for all blobs.

Measured: the whole fleet in about one second. It found 319 plan files
across B's 987 refs against 171 in its working tree, and a plan in A
that exists on a ref and nowhere in the checkout. That visibility
is the point of the index.

> **Superseded detail.** This note originally proposed feeding those
> blobs to `mdsmith.NewMemWorkspace` and then to the `mdsmith extract`
> subcommand. The subcommand route was wrong — a subprocess per file is
> thousands of forks for one walk — and `pkg/mdsmith` turned out not to
> export front-matter extraction anyway. frit imports
> `mdsmith/pkg/markdown` instead and decodes the front-matter block
> itself. See mdsmith issues #796–799.

### Ranking versions

One plan can exist in several versions at once: the copy on its own
lane, the copy on the default branch, and stale copies on old branches.

The first ranking rule tried was "whichever version the most refs
carry". Run against A that reports 98 plans done where the working
tree says 106. Its 391 refs are mostly old lanes branched before the
work finished and never updated again, so the majority is stale by
construction.

The status flip rides the commit that lands the work, so the branch work
lands on is authoritative. Finding it deliberately does not use `HEAD`:
A's main worktree is parked on a feature branch, so `HEAD` names
whatever was last checked out. The cascade asks `origin/HEAD`, then
`main`, then `master`, then `HEAD` as a last resort.

## Multi-host: git is already the bus

The repositories here point at two forges: GitHub and a self-hosted
Gitea. In A the self-hosted remote carries 192 branches against
GitHub's 121, so the private forge is where coordination actually
happens. plan-lane's
`handoff` and `adopt` already move a lane between machines through it,
with no daemon and no central service.

Host state therefore splits in two, and only one half needs anything
new:

- **Durable state** — which plans exist, who holds what, what merged —
  is already in git and readable from any host with one `ls-remote`.
- **Ephemeral state** — which agent is live in which pane right now —
  lives only in each host's herdr socket, and is the one piece needing
  fan-out.

For ephemeral state the answer when a second host arrives is SSH fan-out
rather than a daemon: `ssh <host> herdr agent list`, run concurrently,
cached briefly, with an unreachable host rendered stale rather than
dropped. herdr already supports `--remote <ssh-target>`, so attaching to
a pane spotted elsewhere is a one-liner.

v1 does none of this and reads the local socket only. The point of
writing the topology down is that the durable half needs no work when
the fleet grows, and the ephemeral half is one function swapped from
"read socket" to "read sockets". The host dimension stays in the plan
key so that growth is additive rather than a migration.

## From a board to a prompt, without a prompt UI

Read-only is rung zero, not the ceiling. The problem with growing upward
is that the obvious next step — a box you type a prompt into — is a
surface herdr already owns, and duplicating it would make frit a worse
herdr.

The way out is that **the plan already contains the prompt**. Plans
declare, per phase, the model tier and the gate that proves the work:

```text
| Phase                | Design | Implement | Gate                    |
| 1 boulder count      | haiku  | sonnet    | accept/reject unit test |
| 8 relax bound DESIGN | opus   | sonnet    | silhouette-hole test    |
```

Combined with the `plan-phase` skill — which loads only the front matter
and the single `## Phase N` section — the whole prompt is about twenty
characters: `/plan-phase 2607191320 8`. The model tier comes out of the
table, so dispatch is *typed*: phase 8 asks for opus and gets opus. No
general-purpose orchestrator can do this, because none of them reads the
plan.

Four herdr API calls make the ladder work, all confirmed present in its
schema at protocol 17:

```text
worktree.create  {branch, path, base, label, focus}
agent.start      {kind, name, pane_id, args[], timeout_ms}
agent.prompt     {target, text, wait}
agent.wait       {target, until[], timeout_ms}
```

`agent.start.args` is the important one: the tier from the Execution
table maps straight onto `--model haiku|sonnet|opus`.

| Rung | Verb  | What happens                                    | Text sent |
| ---- | ----- | ----------------------------------------------- | --------- |
| 0    | board | Index and display. The resting state.           | none      |
| 1    | open  | Focus the pane, or attach over SSH.             | none      |
| 2    | nudge | `agent.prompt` into an existing idle lane.      | derived   |
| 3    | start | claim → worktree → agent.start → prompt → focus | derived   |

Three rules keep this from becoming herdr:

- **The tool composes the prompt; the user never writes one.** Sent text
  is always a slash command naming a plan and a phase. When free prose
  is genuinely needed, that is the signal to drop to rung 1 and hand the
  human to the pane.
- **One-way door.** It sends, then hands over. `agent.read` exists in
  the API and frit must never call it. Reading the conversation back is
  exactly how a board grows into a chat client.
- **Dry-run by default.** Every rung above 1 prints the composition it
  would execute; `--go` runs it. Read-only stays the default and the
  escalation stays auditable.

### When something extra must be said

Not every dispatch is bare. The escape hatch is the one git already
established for commit messages: a prefilled template, not an empty box.

```sh
frit start shader-unit --phase 3 --note "skip the VRT case, it's flaky"
frit start shader-unit --phase 3 --edit   # $EDITOR, prefilled
```

A `--note` is a rider on a subject the tool still owns; `--edit` hands
the composed prompt to `$EDITOR` to amend before sending. Neither is a
UI: one is a flag, the other is your editor.

## Discovery is what makes dispatch usable

Dispatch assumes a plan id and a phase number, and in practice you have
neither. Discovery is therefore not a convenience layer. It is also
where the index earns its keep, because every question below needs data
only a cross-repo, cross-branch index has.

| Question                  | Command                 | What it needs                   |
| ------------------------- | ----------------------- | ------------------------------- |
| What can I start now?     | `frit ready`            | dependency DAG + holds + status |
| What should I start next? | `frit pick -n 5`        | ranked candidates nobody holds  |
| Where's that plan about X | `frit find "raymarch"`  | search across branches          |
| Which phase is next?      | `frit next`             | first phase not at ✅           |
| What blocks this?         | `frit show <id> --deps` | upstream DAG walk               |

`frit ready` is the flagship. "Every plan whose dependencies are all ✅,
that nobody holds, on any branch, in any repo" cannot be answered today
without reading 152 files by hand.

### Selectors, not ids

Every command taking a plan should accept three forms, resolved in
order, because typing a ten-digit timestamp is its own friction:

- **An exact id** — `2608062210`. Unambiguous, scriptable, what `--json`
  emits.
- **A slug fragment** — `shader-unit`. Resolves against titles and
  branch names; ambiguity prints candidates and exits non-zero rather
  than guessing. The `plan-phase` skill already accepts "enough of the
  title to resolve it".
- **Nothing at all** — inferred from context. Standing in
  `proj-shader-unit-tests`, `frit next` means "the next phase of the
  plan this worktree is working". That is the cwd join run backwards.

Phase numbers get the same treatment: `--phase` is optional and defaults
to the first phase not at ✅, the rule `plan-phase` already follows.

## Risks

| Risk             | Mitigation                                                                                         |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| Convention drift | Treat the plan convention as optional per repo and degrade to branch-and-agent view                |
| Third registry   | Read holds from refs; delegate every mutation to plan-lane                                         |
| Unreachable host | Never block the board on a dead SSH target; render last-known state with an explicit staleness age |
| Stale index      | Cache with a short TTL keyed on the repo's ref advertisement, not a wall clock                     |

## Scope for the first version

- **One host.** No SSH fan-out in v1, but the host dimension stays in
  the data model and the plan key, so a second machine is a new presence
  source rather than a schema migration.
- **Read-only, as rung zero.** v1 ships rungs 0 and 1. This is a
  starting point, not a ceiling; the ladder above is the growth path and
  never requires a prompt UI.
- **plan-lane keeps the claims.** Holds are read through `ls-remote`,
  the same registry plan-lane writes. No second source of truth and no
  reimplemented race logic.

## Build order

1. **A dedicated repo consuming mdsmith** — see [options.md](options.md).
2. **Index before display.** Get the walk correct across the two
   repositories with full convention, and let one with no `plan/`
   directory exercise the degraded path.
3. **Ship orphan detection early.** It needs only git, pays for itself
   against 80 worktrees, and validates the index without herdr.
4. **Join to herdr last.** It is the smallest piece — one socket call,
   one `rev-parse` per pane — and the only one needing a live server.
5. **Then climb.** Discovery before any dispatch verb: it is what makes
   dispatch usable, and both are still read-only.
