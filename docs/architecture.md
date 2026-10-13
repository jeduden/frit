# Architecture

frit consumes rather than reimplements. Two tools already own a layer
of this problem, and frit is the join between them. It owns one
mutation itself, the claim.

See [How frit and mdsmith fit together](mdsmith-and-frit.md) for the
frit/mdsmith boundary in depth, and [the claiming walkthrough](claiming.md)
for how a hold is minted and read.

## What each tool owns

- **mdsmith** owns markdown, and is imported as a library, not run
  as a subprocess. `pkg/markdown` splits front matter from body and
  hands back the AST, so frit and mdsmith always agree on where a
  document's front matter ends — including the awkward cases, like a
  block scalar containing a line of three dashes. A subprocess per
  file would be thousands of forks for one walk.
  What the public API does *not* expose is `extract`'s CUE schema
  projection and the `deps` link graph; both live in `internal/`.
  If frit needs link-following later, that is the moment to ask for
  them to be promoted, not to hand-roll a second parser.
- **herdr**, where it is installed, owns panes, worktrees and
  prompts. frit reads its socket API for live agent state and hands
  panes back to it for anything interactive. herdr is optional: see
  [Running without herdr](#running-without-herdr).
- **The claim is frit's own.** A hold is a ref name, and frit mints
  it: an empty marker commit pushed with `--force-with-lease`, so a
  hold is atomic across machines and a lost race is caught rather than
  papered over ([internal/claim](../internal/claim)). frit reads holds
  out of the ref list too — which names count is declared per
  repository in its `.frit.yml`, never inferred from a plan id, and
  refs merged into the default branch are excluded so landed work does
  not read as an active claim. Where herdr is installed, frit hands it
  the worktree and the pane the claim stands up; the lease is the one
  thing frit writes to origin.

## Steering is local; coordination is git origin

frit steers agents from their local edits. What an agent has done — the
commits in its worktree, the branch it stands on, the pane it works in —
is a fact of the host it runs on, read there and acted on there.

frit runs as many instances, on many hosts, at once. Nothing on one host
reaches another except through one central place: git origin. The lease
that says who holds a plan, the rescue ref that parks a lane's
divergence, the plan files themselves — all are refs on origin, because a
ref push is the only atom every host can see and race on safely.

So a fact stays where it lives. A local fact — a live pane, an in-flight
merge, a PR open on a pushed branch — is not on origin, and another
host's frit cannot infer it from refs. frit reads such a fact from the
host that has it, over herdr's socket, or asks the agent directly. It
never guesses it from the shared refs, which cannot carry it. This is why
a deserted, unlanded lane and one whose work is open as a PR read alike
to a distant frit: the difference is local, so the resolution is local
too. How one host might learn another's local facts — pull herdr over
ssh, or publish it to origin as a heartbeat — is worked through in
[cross-host presence](research/cross-host-presence.md).

## The one-mutation rule

The rule that keeps those boundaries honest: frit indexes, displays,
and owns exactly one mutation — the claim, because a hold has to be
atomic and a ref push is the only atom git offers. It never edits a
plan, never spawns an agent it does not hand straight back, and never
reads a transcript.

## Running without herdr

herdr is optional. frit takes its herdr-less path only when the
`herdr` executable is not on `$PATH`. That absence is certain, so
every herdr call would fail. A herdr that is installed but does not
answer is not certain to stay down. It keeps the herdr path, and the
lease protocol's own unwind covers it (S60, S61).

Without herdr, each thing herdr did falls to git or goes back to the
caller:

- **Presence** reads as unknown. The board shows `?`, and a takeover
  is protected by the staleness window alone, as when herdr is down.
- **The worktree** is plain git. frit stands a lane up with
  `git worktree add` and tears it down with `git worktree remove`,
  the same calls reap already makes.
- **The session** is never bound. The lease carries none, so no
  live-session veto protects the hold; only the window does.
- **The pane** has no git stand-in. frit dispatches nothing. It hands
  the composed prompt and the lane's path back to the caller, the
  same hand-back a dry-run `start` already prints. A verb that only
  steers a pane refuses, naming herdr not installed.

The lease is untouched by any of this: a claim on a herdr-less host
is the same ref on origin as one anywhere else. Standing a worktree
up changes only this host's own checkout and is never coordinated
through origin, so the one-mutation rule holds.

## The discovery core

The discovery verbs read the fleet index
[internal/fleet](../internal/fleet) builds, and reason over it in
[internal/discovery](../internal/discovery). That package is pure. The
DAG walk, the readiness rule and the selector are tested against an
in-memory fleet with no repository on disk.
