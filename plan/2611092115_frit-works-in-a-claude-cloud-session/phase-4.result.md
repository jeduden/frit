---
n: 4
title: a renamed holder keeps its own lane
status: "✅"
result: true
summary: >-
  A lane now belongs to the holder its newest marker names. A holder
  renamed since its claim, or one that took the lane over, is no
  longer refused from its own lane. C28 covers it.
---
# Phase 4 result

## Handoff

The foreign-lane check reads the marker governing the lease now.

**Verified by.** Three checks:

- C28 on the built frit. Before the change, the release from the
  lane was refused as "held by vm". After it, origin's tip is the
  release.
- Unit tests in the claim package: a lane claimed as `vm`, taken
  over, then renewed under a third name reads as the newest holder
  each time, and a claim older than the lease protocol still reads
  by its `host:` line.
- `go test ./...`, the coverage gate, golangci-lint and
  `mdsmith check .` are clean.
