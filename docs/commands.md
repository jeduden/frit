# Commands

`frit --help` lists every verb, and `frit <verb> --help` its flags.
Grouped by what they do.

## Survey — read the fleet, change nothing

| Verb               | What it does                                                                                                           |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| `repos`            | list repositories and their worktrees                                                                                  |
| `plans [--detail]` | count plan files on every ref, or list them                                                                            |
| `board [--wip]`    | outstanding plans: status, holder, agent; `--columns` picks columns, `description` and `lane` alias `title` and `held` |
| `who`              | which lane has a live agent, read from herdr                                                                           |
| `stale --days N`   | worktrees whose branch has not moved                                                                                   |
| `orphans`          | claims, checkouts and rescue refs that no longer add up                                                                |
| `doctor`           | plans with a semantic gap: missing Goal, tier, Execution row                                                           |
| `drift`            | not-done plans whose work has landed, with the commit evidence                                                         |

## Discover — find the next plan

| Verb             | What it does                                                                        |
| ---------------- | ----------------------------------------------------------------------------------- |
| `ready`          | plans startable now: deps done, nobody holds; `--all` adds files that are not plans |
| `pick [-n N]`    | the same, ranked by how many plans each unblocks; `--go` starts one                 |
| `next <plan>`    | the first phase of a plan not yet done                                              |
| `phase [<plan>]` | the open phase's bundle; runs only from inside the plan's lane                      |
| `show <plan>`    | a plan and everything that blocks it; `--all` adds deps already done                |
| `find <text>`    | search titles and summaries across every ref                                        |

## Lease — hold a plan, or let it go

| Verb             | What it does                                                      |
| ---------------- | ----------------------------------------------------------------- |
| `claim <plan>`   | mint the atomic hold on a startable plan                          |
| `release <plan>` | end this lane's own lease with a release marker                   |
| `yield <plan>`   | end a fenced lane: park its commits to a rescue ref, tear it down |

## Drive — steer a lane up the ladder

The drive verbs are a ladder — ways to move an idle or stuck lane, from
the gentlest to the most forceful. `open` only raises the lane's pane
so you can read it. `nudge` prompts its next open phase back to life.
`message` sends the lane your own words. `start` stands a fresh lane up
from nothing. Climb only as far as a lane needs: read it before you
prompt it, prompt it before you write to it. The rungs that send are
dry runs until `--go` — the
[why](ux-principles.md#an-act-previews-before-it-commits).

| Verb                 | What it does                                                                                                          |
| -------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `open <plan>`        | focus the pane a plan's lane runs in; reads only, sends no text                                                       |
| `nudge <plan>`       | prompt the next open phase into an idle lane                                                                          |
| `message <plan> ...` | send text to a live lane, working or idle                                                                             |
| `start <plan>`       | claim, stand up the worktree, start the agent, send the prompt; `--note` adds a rider, `--edit` opens it in `$EDITOR` |

## Clean and set up

| Verb             | What it does                                                      |
| ---------------- | ----------------------------------------------------------------- |
| `reap [<plan>]`  | tear down what `orphans` reports                                  |
| `init [<dir>]`   | write `.frit.yml` with every default; `--mdsmith` adds the schema |
| `skills [<dir>]` | install the bundled agent skills into `.claude/skills`            |

## Conventions

Three conventions hold across these verbs, each detailed elsewhere. A
`<plan>` is [named three ways](ux-principles.md#naming-a-plan): an id,
a title fragment, or nothing inside its own worktree. Verbs that
compose-and-send are [dry runs until
`--go`](ux-principles.md#an-act-previews-before-it-commits) — `nudge`,
`message`, `start`, `pick`, `reap` — while read verbs, `open` among
them, and the single-ref pushes `claim`, `release` and `yield` act at
once. A refused claim [is not an
error](claiming.md#when-a-claim-is-refused): it prints the reason and
exits 0.

`board`, `ready`, `pick` and `find` take `--sort status|repo|id|held`
and `--reverse`. Tables trim titles to the width on a TTY, or to
`--width N` where none can be measured; global flags sit before or
after the verb.
