# How frit and mdsmith fit together

frit does not parse markdown, lint it, or generate an index of it. It
reads plans, reasons over them, and claims one. Everything to do with
the markdown itself belongs to mdsmith. This page explains where the
line between the two tools runs, why it runs there, and what happens
when frit needs something mdsmith keeps to itself.

## The one rule

frit consumes rather than reimplements. Two tools already own a layer
of this problem. mdsmith owns markdown. herdr owns panes, worktrees and
prompts. frit is the join between them, and it owns exactly one
mutation of its own: the claim.

So the test for any new frit feature is short. If it reads or reasons
over plans, it is frit's. If it parses, lints, or rewrites the markdown
a plan is written in, it is mdsmith's, and frit calls mdsmith for it
rather than growing a second copy.

## What mdsmith owns

- **The parse.** mdsmith's `pkg/markdown` splits a file's front matter
  from its body and hands back the body as an AST. frit imports it as a
  library. So frit and mdsmith always agree on where front matter ends,
  including the awkward cases — a block scalar whose text contains a
  line of three dashes, for one.
- **The schema and the lint.** A plan is validated against
  `plan/proto.md`, the schema template that sits beside real plans.
  `mdsmith check` runs the structural rules and the per-kind schema. A
  malformed plan goes red there, at commit time.
- **The catalog.** `PLAN.md` is generated, not written. A `<?catalog?>`
  directive in it enumerates the plan files into a table, and
  `mdsmith fix PLAN.md` regenerates it. The index is mdsmith's output.
- **The merge of regenerable sections.** Because the catalog is
  generated, two branches that each add a plan conflict on `PLAN.md` by
  construction. mdsmith ships a merge driver that regenerates the
  section during a merge instead of leaving conflict markers.

Two more capabilities live in mdsmith's `internal/` today: `extract`,
which projects a file's front matter through a CUE schema, and `deps`,
the link graph between files. frit does not use them yet.

## What frit owns

- **The index across refs.** frit enumerates plans directly from the
  plan files. `internal/plans` lists every `*.md` under the configured
  plan directory on every git ref, by streaming blobs out of git, and
  `internal/index` parses each one. frit never reads `PLAN.md` to do
  this. The catalog is for a person; frit reads the source.
- **The reasoning.** The dependency walk, the readiness rule and the
  ranking live in `internal/discovery`, over an in-memory model of the
  fleet. That package is pure and never touches a repository on disk.
- **The claim.** A hold is a branch ref frit mints with a
  `--force-with-lease` push, atomic across machines. It is the one
  thing frit writes. See [claiming](claiming.md).
- **The handoff.** frit resolves a plan to a lane and a model tier and
  hands the pane to herdr. It renders no prompt text and reads no
  reply.

## The seam

frit reaches into mdsmith at one point: the parse. It calls
`markdown.Parse` and walks the returned AST for the body sections it
needs — the `## Goal`, and in time the `## Phase` sections and the
`## Execution` table. It walks the same AST mdsmith built, so the two
tools cannot disagree about the document's shape.

frit imports this rather than shelling out to `mdsmith` per file. A
subprocess for every plan on every ref would be thousands of forks for
one walk of the fleet. The library call is one parser, shared.

## When frit needs more than mdsmith exposes

Some of what frit might want — the `extract` CUE projection, the `deps`
link graph — lives in mdsmith's `internal/` and is not part of its
public API. The rule for that case is fixed. frit asks mdsmith to
promote the capability into its public surface. frit does not hand-roll
a second parser or a second schema checker to avoid the ask.

`frit doctor` is the first place this comes up. It reports plans with a
semantic gap — a missing Goal, a phase with no Execution row, a tier
that is not a known model — and it validates through mdsmith rather
than a rule set frit invented. Where the entry point it needs is still
internal, that is the moment to promote it, not to reimplement it.

## herdr, in one paragraph

The other half of the join is herdr. It owns the panes, the worktrees
and the interactive prompts. frit reads its socket for live agent
state, so the board knows which lane is being worked right now, and
hands panes back to it for anything a person types into. frit indexes
and displays; herdr runs the session. The claim is the seam between
them that survives a machine going away, because it lives in git.
