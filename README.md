# frit

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

## Commands

```sh
frit repos              # every repository and its worktrees
frit plans              # plan files found on every ref
frit plans --detail     # ...and which refs carry each one
frit orphans            # claims and checkouts that no longer add up
frit stale --days 21    # worktrees whose branch has not moved
frit init               # write .frit.yml with frit's defaults
```

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
