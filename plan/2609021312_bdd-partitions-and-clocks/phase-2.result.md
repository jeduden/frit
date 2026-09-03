---
n: 2
title: The three verb-level partition rows and the far-forward clock row run for real
status: "✅"
result: true
summary: >-
  S22, S23, S24 and S35 are real Given/When/Then scenarios, none
  @pending, all bound in the existing
  cmd/frit/bdd_partitions_and_clocks_test.go via a second registrar
  kept apart only for the file's own lint budget. S22 and S24 go
  through the real board verb
  against a second, breakable checkout — fetchRemote/staleFetch's own
  degrade, never a fake Runner. S23 drives observeHolds directly
  against a synthetic fleet and an explicit clock, no repository at
  all. S35 corrects Phase 1's own suggestion of an early-firing
  window — the pure discovery functions cannot produce one — and
  instead proves the takeover backoff through a real chain. Every new
  step function carries its own unit test.
---
## Handoff

### What is green

`go test ./cmd/frit -run 'TestFeatures/S(22|23|24|35):'` reports all
four as PASS, none SKIP — alongside S16, S20, S21, S25, S33, S34 from
earlier plans and phases, still green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run ./...` are clean.

- **S22** (observer partitioned): a second checkout, cloned under its
  own `--root` via a new `observerClone` helper (not `cloneAgain`'s
  own throwaway `t.TempDir()` — `board` must actually discover it), is
  seeded a window already three hours matured via the existing
  `seedWindow`. Its `remote.origin.url` is pointed at a path that
  never existed; `board --root <observerRoot> --json` still reports
  the plan's `stale_seconds` and names the failed fetch as a Problem
  ("could not fetch ...; may be stale"), and "box-a"'s own remote and
  origin's tip are both untouched.
- **S24** (asymmetric: push ok, fetch fails): "box-a" renews for real
  in a real lane — a plain win, no fence. Only after that lands does
  the observer clone (so it already carries the fresh tip with no
  fetch needed), then gets cut off the same way as S22's. `board`
  still reports `held: true` for the renewed plan while separately
  naming the fetch failure — the read-side degrade never corrupts a
  write-side result it has nothing to do with.
- **S23** (everyone partitioned, origin up): no repository at all.
  Three synthetic `discovery.Plan`s with string tips are seeded
  windows directly through `observe.Save`, sampled at this section's
  own clock; the clock then advances past `DefaultSampleGap` with no
  new sample, and `observeHolds` is called directly against a
  hand-built `fleet.Result` and `&runtime{git: gitwt.Exec}` — no
  `Coords` entry, so `TakeoverCount` never runs and the discovery
  package's own defaults apply. Every window resets to one sample
  with a void note; no plan reads stale.
- **S35** (clock steps far forward): Phase 1's own handoff suggested a
  single large clock jump would mature a window "early". It cannot:
  `discovery.Observe` voids on any gap over S_max, so a jump that size
  restarts the window at one sample rather than maturing it — the
  same rule any partition trips. The row's real, testable content
  turned out to be the backoff alone. "box-a" holds; the existing
  `observerWatchesTipGoStale` matures a window and takes over exactly
  as S20 does — nothing new there. A second call to the same helper,
  now against "box-b"'s own tip, matures a fresh window of the same
  shape. A new Then step reads `claim.TakeoverCount` off the real
  takeover chain (`k = 1`), computes `threshold = (k+1) *
  DefaultTakeoverWindow`, and asserts the span reads stale under the
  bare window (proving it actually matured) but not under the
  backed-off one — the pair, not either alone, pins the damping.

### Findings

- **Phase 1's own S35 sketch does not fit the pure functions.**
  `discovery.Observe`/`StaleHold` take whatever `now` a caller passes
  and never see wall-clock time; a fast or skewed clock only ever
  changes how much *real* time one logical takeover window
  represents, which these functions cannot distinguish from ordinary
  maturation at their own layer — see this plan's `plan.md`, whose
  own "Verb-level" bucket for S35 also assumed a fake-Runner shape
  that turned out unnecessary. Recorded here so a later plan touching
  the discovery package does not rediscover the same dead end.
- **`fetchRemote`/`staleFetch` already do the work S22 and S24 needed.**
  Neither row needed a fake `gitwt.Runner`: `fleet.Gather`'s own
  `--fetch` step already degrades a failed fetch to a `Problem` and
  falls back to whatever remote-tracking view it already had. This
  mechanism was undocumented in Phase 1's handoff (it never looked
  past the lease API), which is why this phase's own research states
  it explicitly for whichever row next touches read-side
  classification.

### What S36 needs

Two independent `discovery.Window`/clock pairs, both observing the
same tip, on clocks skewed by years from each other — a lease-API-level
row like S35 turned out to be, not a verb-level one: `pcState` needs a
second `window`/`clock` pair (or a small map keyed by host) rather
than the single pair every row through this phase has shared. The
assertion is that both windows converge on the same `StaleHold` answer
despite the skew — nothing compares one host's timestamp against the
other's, only each host's own elapsed time against its own last
sample, the same rule S33 and S34 already pin for a single clock.
