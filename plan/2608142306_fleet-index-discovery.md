---
id: 2608142306
title: The fleet index — discover every repo, worktree and branch
status: "✅"
summary: >-
  Build the index frit is named for: walk a root for git
  repositories, enumerate every worktree and branch, read plan
  files straight out of git objects without checking them out,
  and answer which lanes are orphaned. Read-only throughout.
model: opus
depends-on: []
---
# The fleet index

## Goal

Answer three questions across every repository, branch and
worktree on this machine. What work exists? Where does it live?
Which of it is abandoned? Check nothing out, and mutate nothing.

## Context

Work is scattered by construction. One machine here carries 80
worktrees in a single repository, 10 of them pinned at
`00000000` with nothing checked out, and 313 remote refs. Plans
live in `plan/*.md` on branches that may never have had a
worktree at all.

Nothing currently reports which of those lanes is abandoned,
because answering it needs three facts joined together: the
worktree list, the branch each worktree is on, and whether any
commits ever landed there. Git has all three and no tool asks
for them together.

The index is deliberately the first thing built. Discovery is
what makes every later verb — readiness, dispatch, the board —
possible, and it depends on nothing but git.

## Phase 1: worktrees, parsed from porcelain

Walk a root directory for git repositories, and for each one
enumerate its worktrees by parsing `git worktree list
--porcelain`.

The parser is the whole risk surface. It stays pure, and it is
tested against a fixture for every record shape git emits:

- a normal checked-out worktree
- a detached HEAD
- a worktree with no commit (`0000000000...`)
- a bare repository
- the locked and prunable markers

Porcelain is the only form parsed. Records are separated by
blank lines, and each line is a `key value` pair or a bare flag.
That shape is stable across git versions. The human-readable
form is not.

Ship `frit repos` on top of it, so the phase is observable from
the command line rather than only from tests.

## Phase 2: branches and plan blobs, without a checkout

Enumerate every ref per repository — local, remote-tracking and
tags alike. The scope is all of them on purpose. A plan can sit
on a branch that was never checked out, never merged, and only
ever seen on a peer's remote.

Stream the `plan/*.md` blobs straight out of git objects. The
walk resolves one tree per ref in a single `cat-file
--batch-check`, lists only the distinct trees, and reads every
blob in one `cat-file --batch`. Branches that share a plan
directory share its tree object, so the distinct trees are far
fewer than the refs.

Measured on this machine: the whole fleet in 1.7 seconds. It
found 319 plan files across mdsmith's 987 refs against 171 in
its working tree, and one plan that exists on a ref and
nowhere in any checkout.

## Phase 3: plans parsed through mdsmith

Import mdsmith's `pkg/markdown` and split front matter in
process. A subprocess per file would be thousands of forks for
one walk, and the public parser is the same code mdsmith lints
with, so the two never disagree about where front matter ends.

Parse each distinct blob once, not once per ref. Key the index
as `host:repo:id`; never on `id` alone, because ids collide
across repositories.

One plan can exist in several versions at once. Rank them so
the copy on the default branch wins, because the status flip
rides the commit that lands the work. Ranking by how many refs
carry a version is wrong, and measurably so: old lanes
outnumber the default branch and report 98 plans done where
the branch itself says 106.

## Phase 4: orphans and stale lanes

Join worktrees to holds, and holds to plans. Then report the two
questions nothing answers today. Which held plans have no
worktree? Which worktrees have no hold and no commit?

A **hold** is a claim on a plan, and frit recognises one by
matching ref names against configured patterns. It does not
infer a hold by scanning ref names for a known plan id. A hold
is something a repository declares, not something frit guesses:
inference misreads counter-style ids, where `v0.69` and
`issue-100` look exactly like a claim on plan 69 or 100.

Two consequences to design for.

Patterns are a **list** per repository, not one. Conventions
decorate the id freely — one repository here uses four shapes at
once (`claude/plan-25-x`, `codex/plan-64`, `owner/plan-66-slug`,
`plan-152-slug`), and another matches its own canonical
`plan/<id>-<slug>` on only 68 of the 139 refs that carry an id.
A single pattern would see half the lanes.

Unmatched lanes are **invisible by design**, not a bug. A repo
that declares no pattern reports no holds, and that is the
honest answer for a repo with no convention.

Independently of patterns, a hold must exclude refs already
merged into the default branch. A landed plan's branch still
exists, so without that scan finished work reports as actively
held.

Report three kinds separately rather than as one count. A
claimed lane with no checkout means work was taken and dropped;
a worktree with no commit means a lane was prepared and never
started; a prunable one means the checkout is already gone.
They call for different responses.

Measured on this machine: 10 lanes prepared and never started,
and zero claims without a checkout. That zero was checked
against an independent count of unmerged claim branches rather
than taken on trust.

Measure staleness from the branch tip, not the directory mtime.
Builds, greps and editors touch a directory. None of them mean
work happened.

## Phase 5: the JSON contract

Give every command a `--json` form, and pin the shape with a
golden test, because agents consume this tool as much as people
do.

Both renderings come out of one model in `internal/report`, not
two printers kept in step by hand. A command gathers what it
found into a document and renders it as a table or as JSON, so a
fact added for one is in the other by construction.

Three rules make the JSON a contract rather than a dump. Every
key is always present, so a consumer indexes a field without
first testing for it. A list is `[]` and never null, because
iterating null is an error in most languages and iterating
nothing is not. And a repository frit could not read travels
inside the document, so a consumer reading stdout alone can tell
a clean fleet from one that was never opened.

Two divergences from the table are deliberate. `--detail`
decides how much of the plan index a person is shown, while the
document always carries all of it. And the table drops a
repository with nothing to report, while the document keeps it
with empty sets — "walked and found nothing" is an answer, and
the table cannot give it.

The golden files are built from hand-written fixtures rather
than by walking a repository, so nothing in them moves with the
clock, the machine, or a temporary directory's name. A diff
there is a change to the contract and nothing else.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase                | Design | Implement | Gate that catches a wrong answer                                  |
| -------------------- | ------ | --------- | ----------------------------------------------------------------- |
| 1 worktree porcelain | sonnet | sonnet    | parser unit tests over detached, no-commit, bare and locked cases |
| 2 blobs, no checkout | opus   | sonnet    | integration test builds a real repo, reads a plan off a branch    |
| 3 mdsmith library    | sonnet | sonnet    | id collision test across two repos sharing an id                  |
| 4 orphans and stale  | opus   | sonnet    | fixture repo with a held plan and no worktree, and the inverse    |
| 5 JSON contract      | haiku  | sonnet    | golden-file test over the emitted shape                           |

## Non-goals

- No writes of any kind. No claiming, no worktree creation, no
  prompting. Those are later plans and a deliberate escalation.
- No herdr integration. Agent presence joins in after the index
  is correct, because it is the only part needing a live server.
- No multi-host fan-out. The host dimension stays in the key so
  that adding a machine later is additive, but v1 reads one.

## Tasks

1. Parse `git worktree list --porcelain` into typed records
2. Discover git repositories under a root directory
3. Expose both through `frit repos`, with tests
4. Enumerate refs and stream plan blobs per repository
5. Build the plan index through mdsmith's `pkg/markdown`
6. Read per-repo `.frit.yml`; ship `frit init` to write it
7. Report orphaned and stale lanes
8. Add the `--json` contract and pin it with a golden test

## Acceptance Criteria

- [x] `git worktree list --porcelain` is parsed into typed records,
      covering detached HEAD, no-commit, bare, locked and prunable
- [x] Repository discovery finds every git worktree under a root
      without descending into `.git` or nested checkouts
- [x] `frit repos` prints each repository and its worktrees
- [x] Plan blobs are read from refs with no checkout, covering
      local, remote-tracking and tag refs
- [x] Plans are indexed as `host:repo:id`, parsing each distinct
      blob once and preferring the default branch's version
- [x] Hold patterns are configured per repository in `.frit.yml`,
      as a list, with `frit init` writing the defaults
- [x] Holds exclude refs already merged into the default branch
- [x] Orphaned and stale lanes are reported
- [x] Every command has a `--json` form
- [x] All tests pass: `go test ./...`
- [x] `go vet ./...` is clean
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
