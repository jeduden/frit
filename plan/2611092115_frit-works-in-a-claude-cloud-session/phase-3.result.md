---
n: 3
title: the review's design decisions
status: "✅"
result: true
summary: >-
  Rescue branches are now frit-rescue/, and frit's fetch skips them. A
  committed .frit.yml cannot set the holder. doctor warns through gh
  when GitHub keeps merged branches. The skills open a cloud PR from
  plan/<id>.
---
# Phase 3 result

## Handoff

All seven decisions are applied as phase-3.md records them.

**Verified by.** Five checks:

- C27 parks under `frit-rescue/` through a remote refusing what the
  proxy refuses.
- A real-git test fetches `plan/7` and leaves the rescue branch on
  origin.
- The config tests show a repo `.frit.yml` holder ignored, while the
  user config and `$FRIT_CONFIG` still set one.
- The doctor tests run against a fake `gh`: setting off, setting on,
  `gh` missing, and a non-GitHub origin.
- `go test ./...`, the coverage gate, golangci-lint and
  `mdsmith check .` are clean.

**Found, not fixed.** The user config outranks a `.frit.yml` beside
the work, which docs/configuration.md states the other way round.
That was already so before this plan, and it is left for its own
change.
