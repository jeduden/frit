---
n: 1
title: claim stands its own worktree up without herdr
status: "🔲"
result: false
---
The proving slice. On a host where herdr is not installed, `frit
claim <id>` mints the lease and stands the lane up with git. It does
not refuse. `frit release` from inside that worktree then ends the
lease. This fixes the test approach later tasks copy: the built frit,
run under a `$PATH` holding git alone, against real repositories.

**Read first.** Find out what herdr's `worktree create --branch
--base` checks out when `plan/<id>` already exists on origin as the
minted marker. The git stand-in must leave the lane on the same tip
herdr would, so the persisted token, the beat and S96's divergence
check read it alike. The likely answer is `git -C <repo> worktree add
-b plan/<id> <lane> <minted tip>`. Record what was found in the
result's Handoff.

**RED.** Rewrite C15 in
[commands.feature](../../features/commands.feature). It keeps its id
and becomes a success:

- Given a plan nobody has ever held, and herdr is not installed on
  this host.
- When the built frit runs `claim 7`, it reports the plan held and
  names the lane's path.
- Then origin's `plan/7` carries a claim marker with no session.
- And the lane's worktree exists on `plan/7`, at the minted tip,
  holding the plan's token.
- When the built frit runs `release` from that worktree, origin's
  `plan/7` carries a release marker.

Update C15's row in
[command-scenarios.md](../../docs/research/command-scenarios.md) to
the new outcome. Do not delete it: the refusal it pinned is
withdrawn, not lost. C16 stays as written, since `pick --go` still
refuses until Task 2 lands.

Unit tests come first, red, for each new function. Cover the stand-in
reporting success. Cover a `worktree add` that fails, which must
release the lease through the same unwind herdr's failure takes,
so origin ends on a release marker.

**GREEN.** Four changes, each behind its red test:

- In claim's stand-up, branch on the missing-herdr probe. herdr
  absent: `git worktree add` at the lane path claim already computes,
  then persist the token as today. herdr present: today's
  `worktree create`, unchanged.
- claim's pre-mint refusal no longer fires when herdr is missing. A
  plan claim would refuse anyway keeps its own reason, as before.
- `start --go` and `pick --go` keep their refusal: they still need a
  pane until Task 2. Their shared preflight must not lose it when
  claim's is removed.
- The claim document carries no herdr problem on this path, since
  nothing failed. Its `--json` is unchanged in shape.

**Docs and skill.** In the [README](../../README.md), move claim out
of "herdr must be running". In the `plan-pick` skill, replace the
line explaining `herdr not found; nothing claimed` for `claim`; keep
it for `pick --go`. Edit
`internal/skills/assets/plan-pick/SKILL.md` and its `.claude/skills`
copy together.

**BDD coverage.** This changes the lease protocol: a hold minted
with no bound session, on a host that can never supply one, is
protected by the window alone. Check
[lease-protocol.md](../../docs/research/lease-protocol.md) first.
S61 covers herdr down at *observation*, not a hold minted with no
session. If no row already says this, add the next free `@S<n>`: a
takeover from another host is refused as held until the window
matures, then wins. Follow [development.md](../../docs/development.md)'s
matrix procedure. C15 is the command row. S60 and S61 must pass
unchanged.

**Gate.** The phase closes when all of these hold:

- The rewritten C15 and the S row pass against the built binary.
- With the probe branch reverted, C15 fails on the old refusal.
- `go test ./...` and golangci-lint are clean.
- Run by hand under a git-only `$PATH`, the built frit claims and
  releases. The `plan-pick` skill's line matches what it prints.
