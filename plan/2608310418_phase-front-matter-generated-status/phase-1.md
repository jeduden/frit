---
n: 1
title: A phase file carries its status and a generated table shows it
status: "✅"
---
Prove the whole idea on the smallest slice. A phase file holds its own
`{n, title, status}`. The linter requires it. A folder plan's
`## Phases` table is generated from it. Every existing plan still passes
`mdsmith check .`.

Assumes: the `phase-spec` kind in [.mdsmith.yml](../../.mdsmith.yml)
pins only the filename today. The `<?catalog?>` directive already
renders [PLAN.md](../../PLAN.md) from front matter over a glob. The plan
structure is schema'd by [plan/proto.md](../proto.md).

Value: a phase file becomes the single home of its status, and a
derived table shows it. A hand-flipped ledger can then no longer drift.
That is the payoff issue 110 asks for.

RED. Add a throwaway phase-spec fixture at a `plan/*/phase-*.md` path
with no front matter. Confirm it passes `mdsmith check` at HEAD, so
requiring front matter is a real change. Separately, confirm this plan's
`plan.md` renders no `## Phases` table today. Capture both as the red
evidence. There is no Go test here — the linter is the instrument.

GREEN. In [.mdsmith.yml](../../.mdsmith.yml), add
`required-frontmatter: [n, title, status]` to the `phase-spec` kind.
Add an *optional* `## Phases` section to [plan/proto.md](../proto.md).
It carries a `<?catalog?>` over the relative glob `phase-*.md`, sorted
`numeric:n`, with a `| # | Status | Phase |` table whose row links
`phase-{n}.md`. Keep the scaffold copy
[internal/scaffold/assets/proto.md](../../internal/scaffold/assets/proto.md)
byte-equal, so the pinning Go test stays green. Give this plan's own
`phase-1.md` its front matter. Add the `## Phases` section to this
`plan.md` so the table renders. Delete the throwaway fixture.

The `## Phases` section must be optional in the schema. Every existing
flat and ledgered plan lacks it and must still pass. If the schema
mechanism forces the section on all plans, that is the seam to solve
before green. Do not weaken an unrelated rule to reach it.

Gate: `mdsmith` renders the `## Phases` table in this `plan.md` from the
phase front matter. A phase-spec file missing front matter now fails
`mdsmith check`. `mdsmith check .` is clean across every plan on disk.
`go test ./...` and the proto-copy pin stay green.
