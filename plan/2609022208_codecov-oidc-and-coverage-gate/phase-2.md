---
n: 2
title: The project status can only drop 0.5%
status: "🔲"
result: false
---
Turn the confirmed upload into the gate. `codecov.yml` grows the
`coverage.status` stanza. The Codecov project check becomes a
required check on `main` — the last piece the Goal names.

**Assumes.** [Phase 1](phase-1.result.md) landed on
[PR #154](https://github.com/jeduden/frit/pull/154): the `test` job
uploads `cover.out` to Codecov over OIDC on every push and same-repo
PR, confirmed by a real run whose commit shows a complete report on
Codecov. `codecov.yml` today carries only `codecov: notify:
wait_for_ci: true` and a `comment:` block.

**RED / GREEN — the config.** A YAML edit has no Go test; its red is
the checks the change must satisfy, run before and after, plus the
PR's own Codecov check appearing and reading correctly once pushed.

- Add to `codecov.yml`:
  - `coverage.status.project.default`: `target: auto`,
    `threshold: 0.5%`, `if_no_uploads: success`,
    `if_not_found: success` — a same-repo run gates on the real
    number; a fork PR (which never uploads) still posts green.
  - `coverage.status.patch.default`: `target: auto`, `threshold: 0%`,
    `if_no_uploads: success`, `if_not_found: success` — new code
    carries its own coverage; the project average alone cannot be
    gamed by pairing untested lines with well-covered ones.
  - No `changes`, `ignore:`, `flag_management` or
    `component_management` stanza — frit is a single-language Go tree
    with no vendored code and one upload flag, unlike mdsmith's.
- Push to [PR #154](https://github.com/jeduden/frit/pull/154) and
  read the Codecov `project`/`patch` checks it posts — not just that
  they exist, but that `project` reads the base commit's coverage as
  `target: auto`'s baseline.
- Make the Codecov check required on `main`'s branch protection
  (`required_status_checks.contexts`), alongside the existing `test`,
  `lint`, `markdown`, `workflows`. **Confirm with the user before
  changing branch protection** — it is a shared repository setting,
  not a file this plan's tree can gate red/green.

**Baseline established.** PR #154 merged
(`e90c2d65eccb98d8579655fa9e7a8615e7be4241`); `main`'s CI passed and
Codecov's report for that commit is `complete` at 88.21%. This PR
pushes a second, small commit from the same lane to open a follow-up
PR against `main` and confirm the `project`/`patch` checks post for
real against that baseline.

**Discovered: no status ever posts, on any commit, in this repo.**
The first theory — no baseline before PR #154 merges — turned out
incomplete. [PR #156](https://github.com/jeduden/frit/pull/156)
opened after the merge. It shows Codecov comparing against a real
base: `base_totals` is populated at 88.21%, matching `main`. Still, no
`project`/`patch` status or check-run appears on its commit.
`gh api repos/jeduden/frit/commits/<sha>/status` stays `total_count:
0` throughout, and `.../check-suites` lists only `github-actions` and
`claude` — no `codecov` entry. The same query against
`jeduden/mdsmith`, which gates nothing on it either but does receive
one, shows a completed `codecov` check-suite on its commits. Uploads
work in both repos (OIDC does not depend on it), but only `mdsmith`
has a `codecov` check-suite at all — the GitHub App that posts
statuses/checks back is authorized on `jeduden/mdsmith` and is not on
`jeduden/frit`. That is a per-repository grant on GitHub's side
(Settings → Applications → Installed GitHub Apps → Codecov →
Configure → add `frit`, or the equivalent toggle in Codecov's own
repo settings), not something a commit in this tree can fix.

**Blocked.** Gate items 3 and 4 need that grant before they can be
attempted at all — requiring the check now would deadlock every merge
on a context that structurally cannot appear. Reported to the user
rather than guessed at further.

**Gate.** Four checks; the last two are blocked on the GitHub App
grant above:

1. `codecov.yml` validates: `curl -X POST --data-binary @codecov.yml
   https://codecov.io/validate` returns the parsed config. ✅
2. `go test ./...`, `golangci-lint`, and `mdsmith check .` stay green
   — this change touches no Go. ✅
3. Codecov posts both `project` and `patch` checks with a real
   baseline on a same-repo PR — not `if_not_found: success` firing
   for lack of one. **Blocked** on the Codecov App's `frit` grant.
4. `main`'s branch protection then lists the Codecov check under
   `required_status_checks.contexts` — confirmed with the user first.
   **Blocked** on gate 3.

Then the full `go test ./...` and lint stay green — this change
touches no Go — and `mdsmith check .` passes on the plan.
