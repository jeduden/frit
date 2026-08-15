---
summary: >-
  Build, extend or adopt — why mdsmith needed no generalisation and still
  was not the right home, a survey of 14 agent orchestrators scored
  against six requirements, and the three-layer model that explains why
  none of them competes with a work index.
---
# Build, extend, or adopt

Three options were weighed for where the fleet index should live:
extend mdsmith, build a dedicated tool, or adopt something existing.

## mdsmith does not know what a plan is

The worry behind extending mdsmith was that it would hard-code one
repository's plan convention into a general-purpose linter. Checking
the source, that concern does not apply. mdsmith has no `depends-on`,
no plan status vocabulary, and no task semantics of any kind. The whole
convention is declared in per-repo config:

```yaml
# .mdsmith.yml — the entire definition of "a plan"
kinds:
  plan:
    path-pattern: "plan/{proto.md,[0-9][0-9]*_*.md}"
    rules:
      required-structure:
        schema: plan/proto.md

  unique-frontmatter:
    field: id
    include: ["plan/*.md"]
```

A path pattern, a schema template, and a uniqueness rule. Any task
system storing tasks as markdown with front matter already works with
mdsmith today; Backlog.md's `backlog/tasks/*.md` would need a config
block and nothing else. What would *not* work is a task system that is
not markdown at all — Beads stores JSONL, Jira stores rows in someone
else's database.

So "generalise mdsmith to any task system" was not work that needed
doing. The generalisation pressure belongs on the new layer instead, and
that argues for keeping it out of mdsmith rather than inside it.

## The three options

### Option 1 — extend mdsmith: rejected

Not because of generalisation, which dissolved above, but because of
what the fleet layer does. It shells out to git plumbing across many
repositories, opens a unix socket to a terminal multiplexer, and
eventually fans out over SSH. None of that belongs in a markdown linter
that advertises no outbound network calls at runtime and ships through
eleven release channels.

mdsmith's own architecture docs draw exactly this boundary. Adding herdr
socket code would be the first thing a SOLID audit flags, and every
release channel would carry a dependency none of its users want.

### Option 2 — a dedicated tool in a dedicated repo: chosen

The fleet index is its own concern with its own release cadence, and
mdsmith exposes a public engine API, so the new tool never
re-implements markdown.

> **Refined since.** This note originally proposed driving mdsmith as a
> subprocess (`extract`, `query`, `deps`) to avoid depending on its
> `internal/` packages. That was wrong at scale: a subprocess per file
> is thousands of forks for one walk. frit imports
> `mdsmith/pkg/markdown` instead — a public package with a stated
> compatibility policy — and decodes front matter itself. The
> capabilities that remain CLI-only are filed as mdsmith issues #796
> (front matter, the blocking one), #797 (deps/backlinks), #798
> (extract) and #799 (query).

### Option 3 — adopt something existing: partial, steal ideas

The category is crowded and moving fast. Fourteen tools were surveyed
and several are genuinely good at the agent-fleet half; `repomon` spans
repositories, branches and worktrees from a single TUI. But every one of
them is an *orchestrator* whose job is spawning and steering agents in
worktrees. Not one indexes task or plan markdown across git branches.
Two independent passes over the curated directories confirmed the same
gap.

## The survey

Scored against the six requirements. Ratings come from published
documentation and project descriptions, not hands-on testing — a
shortlist filter, not a benchmark.

| Tool                   | Multi-repo | Multi-host | Plans×branches | MD links | Task DAG | Read-only | Shape                             |
| ---------------------- | ---------- | ---------- | -------------- | -------- | -------- | --------- | --------------------------------- |
| repomon                | ●          | ○          | ○              | ○        | ○        | ○         | Rust TUI, tmux-backed fleet       |
| ai-maestro             | ●          | ●          | ○              | ○        | ○        | ○         | dashboard across machines         |
| diri                   | ●          | ●          | ○              | ○        | ○        | ○         | worktrees or remote hosts         |
| thurbox                | ◐          | ●          | ○              | ○        | ○        | ○         | TUI, SSH sessions, review view    |
| agents-cli             | ●          | ●          | ○              | ○        | ○        | ○         | SSH fleet dispatch, session index |
| vibe-kanban            | ◐          | ○          | ○              | ○        | ○        | ○         | card = worktree + agent           |
| agent-console          | ◐          | ○          | ○              | ○        | ○        | ○         | web console over worktrees        |
| Conductor              | ○          | ○          | ○              | ○        | ○        | ○         | Mac app, one worktree per agent   |
| cli-agent-orchestrator | ●          | ◐          | ○              | ○        | ○        | ○         | AWS, tmux, flows + scheduling     |
| Backlog.md             | ○          | ○          | ◐              | ○        | ◐        | ○         | markdown tasks + kanban, one repo |
| Beads (bd)             | ◐          | ○          | ○              | ○        | ●        | ○         | git-backed graph tracker, JSONL   |
| herdr *(ours)*         | ●          | ◐          | ○              | ○        | ○        | ○         | workspace manager + socket API    |
| plan-lane *(ours)*     | ○          | ●          | ●              | ○        | ●        | ◐         | ref-name holds, one repo          |
| mdsmith *(ours)*       | ○          | ○          | ○              | ●        | ●        | ●         | markdown engine, one checkout     |

Legend: ● yes, ◐ partial or implied, ○ no.

The column that stays empty is *plans × branches*, and it is the column
the whole requirement turns on. The three tools already on this machine
cover more of this matrix than any single third-party product, which is
the strongest argument against adoption: switching would trade a
capability we have for one we would rebuild.

## Why the collisions are not competitors

Five of six candidate names were already taken by projects in this
space, which looks like a crowded market until the claimants are sorted
by what they operate on. The ecosystem has three layers, and almost
everything shipping today sits in the first two.

| Layer      | Operates on                                                      | Who is there                                                                     |
| ---------- | ---------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Intra-run  | Individual actions inside one agent run — gate, approve, restart | a5c-ai/babysitter, thomasgauthier/babysitter, caty-ai/sitter, yusukeshib/babysit |
| Session    | Panes, worktrees, spawning and attaching                         | herdr, repomon, Conductor, vibe-kanban, agent-console                            |
| Work index | What work exists across repos, branches and hosts; who holds it  | *empty*                                                                          |

The four projects sharing the name "babysitter" are all intra-run
supervisors, the layer furthest from a work index. They differ mainly in
how tightly they hold the leash: a5c-ai enforces a JavaScript process
definition with breakpoints and quality gates; thomasgauthier
intercepts agent requests for approve/steer verdicts; caty-ai/sitter is
a watchdog restarting long-running commands; yusukeshib/babysit is a PTY
wrapper. None indexes anything, and none spans repositories, branches or
hosts.

## Ideas worth taking without taking the tool

- **Beads' explicit relation types.** frit's plans have a single
  `depends-on` edge. Beads distinguishes `blocks`, `depends_on`,
  `parent-child` and `relates_to` — the difference between "cannot
  start" and "should read first" is worth encoding once there are 105
  edges to render.
- **repomon's needs-you triage.** A fleet view sorted by which lane is
  blocked on a human rather than by repository. herdr already reports
  the status that would drive it.
- **Backlog.md's zero-config default.** Point it at a folder and it
  works. The index should degrade to branch-and-agent view for
  repositories with no `plan/` directory, as one here requires.
- **Nobody's read-only mode.** Every surveyed tool is a controller. A
  board that can be handed to an agent with no risk of it spawning work
  is unusual, and cheap to preserve if it is a constraint from day one.

## Sources

- [awesome-agent-orchestrators][aao]
- [awesome-cli-coding-agents][acca]
- [repomon][repomon]
- [Backlog.md][backlog]
- [Beads][bd]
- [agent-console][console]
- [Augment Code orchestrator roundup][augment]

[aao]: https://github.com/andyrewlee/awesome-agent-orchestrators
[acca]: https://github.com/bradAGI/awesome-cli-coding-agents
[repomon]: https://github.com/AliHamzaAzam/repomon
[backlog]: https://github.com/MrLesk/Backlog.md
[bd]: https://betterstack.com/community/guides/ai/beads-issue-tracker-ai-agents/
[console]: https://github.com/ms2sato/agent-console
[augment]: https://www.augmentcode.com/tools/open-source-agent-orchestrators
