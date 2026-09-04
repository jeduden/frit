# frit

[![codecov][codecov-badge]][codecov-project]

A command and control CLI over plans, worktrees, hosts and agents.

Frit is prepared, pre-fused material that waits in the batch until it
is melted into the work. This tool is the same thing for plans: it
indexes work that already exists — across every repository, every
branch and every machine — and tells you which of it is ready.

## What it does

One register over work that is scattered by construction:

- **Indexes plans across branches** without checking them out, by
  streaming blobs out of git.
- **Joins to live agents** so the board knows which lane is being
  worked right now, and by what.
- **Answers what is ready** using the plan dependency graph, the
  claim refs, and the status ledger together.

## What it does not do

It never owns a conversation. Dispatch is a lookup and a handoff:
frit resolves a plan to a lane and a model tier, hands the pane to
the agent that already runs there, and steps back. It renders no
text input, and it never reads a transcript back.

## How it coordinates

frit steers agents from their local edits, and it runs as many
instances, on many hosts, at once. The one place they all coordinate is
git origin. The claim that holds a plan, the rescue ref that parks a
lane, and the plan files themselves are all refs pushed there — a ref
push is the only atom every host can see and race on safely. A fact that
never reaches origin is local: a live pane, an in-flight merge, a PR
open on a pushed branch. frit reads such a fact from the host that has
it, or asks the agent; it never infers it from the shared refs.

## Built on mdsmith and herdr

frit consumes rather than reimplements. It does not parse markdown,
lint it, or generate an index of it — mdsmith owns all of that, and
frit imports it as a library. It does not run panes — herdr owns those,
and frit reads its socket and hands panes back. frit is the join, and
it owns exactly one mutation of its own: the claim. Where the line runs
between frit and mdsmith, and why, is written up in
[how frit and mdsmith fit together](docs/mdsmith-and-frit.md).

## Install

From source, with Go 1.25 or newer:

```sh
go install github.com/jeduden/frit/cmd/frit@latest
```

Or download a signed binary from the
[releases](https://github.com/jeduden/frit/releases) page and check it
against the repository before use:

```sh
gh attestation verify frit-linux-amd64 -R jeduden/frit
```

## Commands

The board — what exists and what has gone wrong:

```sh
frit repos              # every repository and its worktrees
frit plans              # plan files found on every ref
frit plans --detail     # ...and which refs carry each one
frit orphans            # claims, checkouts and rescue refs that no longer add up
frit stale --days 21    # worktrees whose branch has not moved
frit who                # which lane has a live agent on it
frit board              # outstanding plans: status, holder, agent, machine
frit board --wip        # ...only the ones in progress
```

Discovery — what to start, and what stands in the way:

```sh
frit ready              # plans startable now: deps done, nobody holds
frit pick -n 5          # ...ranked by how much each unblocks
frit next <plan>        # the first phase of a plan not yet done
frit show <plan>        # what blocks a plan
frit find raymarch      # search titles and summaries across every ref
frit init               # write .frit.yml with frit's defaults
```

A `<plan>` is an exact id, a slug fragment matched against titles and
branches, or nothing at all — inferred from the worktree you are
standing in.

The list commands — `board`, `ready`, `pick` and `find` — take
`--sort` to reorder by `status`, `repo`, `id` (creation time) or
`held`, and `--reverse` to flip it. Without `--sort` each keeps its own
order; `--reverse` alone turns that order end to end.

```sh
frit board --sort id --reverse   # newest work first
frit board --sort held           # claimed lanes first
```

`frit board` shows host, repo, id, status, held, agent and title.
`--columns` narrows that to the ones you name, in that order. Use
`description` for the title and `lane` for who holds it.

```sh
frit board --columns id,description   # just the plan and what it is
```

Dispatch — climb from the board to a running lane:

```sh
frit open <plan>        # focus the pane the plan's lane runs in
frit nudge <plan>       # dry-run the phase prompt; --go sends it
frit claim <plan>       # mint frit's own atomic hold on a startable plan
frit start <plan>       # compose the whole escalation; --go runs it
```

`open` and `nudge` send nothing you did not compose: the text is always
the slash command `/plan-phase <id> <phase>`, and `nudge` is dry-run
until `--go`. `claim` mints the hold as a git ref — an empty marker
commit pushed with `--force-with-lease`, so a hold is atomic across
machines and a lost race is caught rather than papered over.

`start` is the full rung. It mints the claim, then hands the worktree,
the agent at the plan's tier, and the pane to herdr. It is dry-run until
`--go`. Use `--note` to add a rider to the prompt, or `--edit` to amend
it in `$EDITOR`.

[How claiming works](docs/claiming.md) covers how a lease is made and
kept alive, and how two machines avoid taking the same plan. It also
covers a stale or fenced lease — found, taken over or yielded — and a
landed one, scavenged automatically.

## What is hidden by default

Two things are held back so the common view stays quiet, and `--all`
brings both back. A dependency that is already done blocks nothing, so
`show` lists only the open blockers. A file in a plan directory with no
front matter is not a plan, so it is not reported as a problem.

```sh
frit show <plan> --all  # every dependency, done ones included
frit ready --all        # ...and surface files that are not plans
```

## Fitting the terminal

The board and the plan lists trim their titles to the terminal width so
a row never wraps. This happens only when the output is a terminal; a
pipe or a redirect gets the full, stable text. Where there is no
terminal to measure — behind a pager, or under a harness that indents
the output — pass the width to use:

```sh
frit board --width 100  # fit to 100 columns regardless of detection
```

Global flags like `--root`, `--width` and `--json` may sit before the
verb or after it: `frit --root ~/git board` and `frit board --root
~/git` are the same.

## JSON

Every command takes `--json` and answers with a document instead of a
table, because agents read this tool as much as people do. Both come
from one model, so they never disagree.

```sh
frit orphans --json | jq '.repos[] | select(.unstaffed | length > 0)'
```

Three rules make it something to write against. Every key is always
present. A list is `[]` and never null. A repository frit could not
read is carried in `problems`, not printed beside the output, so
stdout is the whole report.

## Per-repository settings

Each indexed repository describes its own conventions in a `.frit.yml`
committed beside its plans, so they travel with the project. `frit
init` writes it; a repository without one gets the defaults.

```yaml
plan-dir: plan

# Ref names that count as a claim on a plan. {id} is the plan id,
# * stops at a slash, ** does not. List every shape in use.
holds:
  - "plan/{id}-*"
```

## Configuration

The fleet root is typed once, not on every invocation. Settings
resolve most-specific first:

```sh
frit repos --root ~/git     # 1. the command line
export FRIT_ROOT=~/git      # 2. the environment
```

```yaml
# 3. .frit.yml beside the work, or $FRIT_CONFIG
# 4. $XDG_CONFIG_HOME/frit/config.yml
root: /home/you/git
```

## Status

Early. See [PLAN.md](PLAN.md) for what is planned and what has
landed.

## License

[MIT](LICENSE)

[codecov-badge]: https://codecov.io/gh/jeduden/frit/graph/badge.svg
[codecov-project]: https://codecov.io/gh/jeduden/frit
