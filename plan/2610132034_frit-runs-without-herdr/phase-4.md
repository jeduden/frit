---
n: 4
title: a harness session publishes its lane through its own branch
status: "✅"
result: false
---
A cloud agent session has no herdr, and its harness fixes the branch
it may publish, such as `claude/<name>`. Since phases 1–3 it can
claim a plan, or start one and get the prompt handed back. That
stands a git lane on `plan/<id>`. This phase teaches it to work there
and publish through its own branch. frit's protocol does not change.

**The flow.** Four steps, the last one frit already handles:

1. From its own checkout, the session runs `start <id> --go --json`
   (or `pick --go --json`). The handoff is `handed-back`, and `lane`
   names the worktree.
2. It works in `lane`, committing on `plan/<id>`, and pushes with
   plain `git push origin plan/<id>`. Each push renews the lease.
3. To publish, it pushes the same tip to its harness branch with
   `git push origin plan/<id>:<harness-branch>`, and opens or updates
   the PR from there.
4. When that branch merges, the lease tip is in `main`: by ancestry
   after a merge commit, or by content after a squash. frit reads the
   plan as landed through its existing evidence.

The copy fast-forwards only if the harness branch is still an
ancestor of the lane, so the session claims before committing on its
harness branch. A copy git refuses as non-fast-forward is reported,
never forced.

**No new verb.** The copy is the agent's own push, like every work
push. frit's one mutation stays the lease.

**RED.** C21, a command scenario against the built frit under a
git-only `$PATH`:

- `start 7 --go --json` hands back.
- A commit made in the lane is pushed to `plan/7`, then to
  `claude/session`.
- Before the merge, `board --json` shows plan 7 held.
- `claude/session` is merged into origin's `main` with a merge
  commit.
- After a fetch, `board --json` shows plan 7 not held.

Before any skill text is written, C21 is run once to confirm the flow
works with no frit change. A failure there is a bug to fix first.

**Docs and skill.** Three edits ride the scenario:

- `plan-phase` gains a line for a session whose harness fixes its
  branch: work in the lane, push `plan/<id>`, then copy the tip to
  the harness branch.
- `plan-pick` and `plan-start` say to `cd` into `lane` on a
  hand-back.
- [claiming.md](../../docs/claiming.md) stops saying yield tears down
  only through herdr, and states the publish-through-a-harness-branch
  flow beside "Work rides the same ref".

**BDD coverage.** C21 is command-level: one host, existing landed
evidence. It adds no lease-protocol row, because the lease is
untouched.

**Gate.** C21 passes. `go test ./...` and golangci-lint are clean.
The skills' lines match the commands C21 runs.
