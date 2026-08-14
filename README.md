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

## Status

Early. See [PLAN.md](PLAN.md) for what is planned and what has
landed.

## License

[MIT](LICENSE)
