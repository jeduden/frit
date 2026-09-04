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

**Discovered: no status posts pre-merge on a first integration.**
`main` has never carried a Codecov report — this plan is what starts
one. Codecov's own PR comment on #154 states outright: "Once you
merge this PR into your default branch, you're all set!" No
`project`/`patch` status or check-run appears on #154's commits.
`gh api repos/jeduden/frit/commits/<sha>/status` stays `total_count:
0`. Requiring the check now, before it has ever posted, would make
GitHub wait on a context that cannot appear on this PR's own commits.
That would deadlock the merge that is supposed to create the
baseline. So the gate below splits into a same-PR half (config
validates, no Go/lint regression) and a post-merge half (the check
posts for real, then goes into `required_status_checks.contexts`).

**Gate.** Four checks; the last two happen after PR #154 merges:

1. `codecov.yml` validates: `curl -X POST --data-binary @codecov.yml
   https://codecov.io/validate` returns the parsed config.
2. `go test ./...`, `golangci-lint`, and `mdsmith check .` stay green
   — this change touches no Go.
3. On the first push to `main` after merge (or the next same-repo PR),
   Codecov posts both `project` and `patch` checks with a real
   baseline — not `if_not_found: success` firing for lack of one.
4. `main`'s branch protection then lists the Codecov check under
   `required_status_checks.contexts` — confirmed with the user first.

Then the full `go test ./...` and lint stay green — this change
touches no Go — and `mdsmith check .` passes on the plan.
