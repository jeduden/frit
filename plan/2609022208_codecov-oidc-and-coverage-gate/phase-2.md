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

**Gate.** Three checks:

1. `codecov.yml` validates: `curl -X POST --data-binary @codecov.yml
   https://codecov.io/validate` returns the parsed config.
2. On PR #154, Codecov posts both `codecov/project` and
   `codecov/patch` checks (or their configured names), and the
   `project` check's baseline is the PR's base commit — not
   `if_not_found: success` firing because no baseline was found.
3. `main`'s branch protection lists the Codecov check under
   `required_status_checks.contexts` — confirmed with the user first.

Then the full `go test ./...` and lint stay green — this change
touches no Go — and `mdsmith check .` passes on the plan.
