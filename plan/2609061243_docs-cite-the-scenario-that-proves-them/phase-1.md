---
n: 1
title: claiming.md and the matrix point at each other
status: "🔲"
result: false
---
Wire claiming.md to the scenarios that prove it, and the matrix to its
feature files. The reader crosses from a lease behavior in prose to
its running scenario in one hop. This fixes the citation shape the
later docs copy.

**BDD coverage.** None applies. This phase adds citations and links to
existing scenarios; it writes no new scenario. Its gate is the
bijection test staying green and the links resolving.

**Assumes.** [claiming.md](../../docs/claiming.md) has sections that
each describe a lease behavior the S-matrix catalogs: two machines at
once (races), staleness and takeover, liveness veto, self-resume,
fencing and yield, when a claim is refused.
[lease-protocol.md](../../docs/research/lease-protocol.md) holds those
S-rows, grouped in sections (Staleness, Fencing, Liveness precedence,
Self-resume, Scavenge, Yield, and the lifecycle groups). Per
[development.md](../../docs/development.md), each matrix section maps to
one feature file under [features/](../../features). The tags and rows
are held in bijection by `go test ./internal/scenario`.

**Value.** The doc that narrates claiming — the behavior most densely
catalogued by the matrix — becomes traceable to the tests that prove
it. A reader checking "does frit really refuse a stale hold" reaches
the scenario; a scenario that changes names the paragraph to revisit.

**RED.** No test turns red — this is prose citation. Guard against a
dead citation instead: the acceptance is that every id claiming.md
names is a real matrix row, which the existing bijection gate already
enforces for the feature side and which a manual grep confirms for the
prose side. Note the current state first: `go test ./internal/scenario`
green, claiming.md citing zero ids.

**GREEN.** In claiming.md, where a section asserts a concrete behavior,
name the scenario that proves it — its id inline, and a link to the
feature file for the section. Match each claiming.md section to its
matrix section and cite the ids there: for example the refusal
behaviors to their rows, fencing and yield to the Fencing and Yield
sections, self-resume to Self-resume. In lease-protocol.md, add a link
from each matrix section to the feature file that holds its scenarios,
so the matrix and its executable form are one click apart.

**Guard the edges.** Cite only where a scenario genuinely proves the
claim; leave the conceptual framing uncited. Do not cite an id that
names no row — the bijection gate covers the feature side, but a prose
typo like `S99` would not be caught, so confirm each cited id against
the matrix by grep. Do not touch a research note's dated body beyond
the section-to-feature link.

**Gate.** Every id claiming.md cites resolves to a real row in the
matrix (confirmed by grep against lease-protocol.md and
command-scenarios.md). Each matrix section links a real file under
features/. `go test ./internal/scenario` and `mdsmith check .` are
green, and every added link resolves.
