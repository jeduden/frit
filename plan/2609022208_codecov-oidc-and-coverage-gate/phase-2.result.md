---
n: 2
title: The project status can only drop 0.5%
status: "✅"
result: true
summary: >-
  `codecov.yml` gates `project` at a 0.5% drop and `patch` at 0%; both
  post as real GitHub checks, and the Codecov check is now required on
  `main`.
---
## Handoff

**Done.** `codecov.yml` carries a `coverage.status` stanza:
`project.default` at `target: auto`, `threshold: 0.5%`; `patch.default`
at `target: auto`, `threshold: 0%`; both with `if_no_uploads: success`
and `if_not_found: success` so a fork PR, which never uploads, still
posts green. `main`'s branch protection
(`required_status_checks.contexts`) now lists `codecov/project` and
`codecov/patch` alongside `test`, `lint`, `markdown`, `workflows`.

**Two real obstacles, both cleared.** First, `main` had never carried
a Codecov report, so the very first PR (#154) couldn't get a real
`project`/`patch` comparison — Codecov's own comment said as much
("Once you merge this PR into your default branch, you're all set!").
Merged #154 to establish the baseline (88.21%, commit `e90c2d6`).
Second, even against that baseline,
[PR #156](https://github.com/jeduden/frit/pull/156) still showed no
status or check-run at all — `jeduden/frit` had no `codecov`
check-suite on any commit, while `jeduden/mdsmith` did.
Uploads work either way (OIDC doesn't need the GitHub App), but
writing checks back does, and the Codecov App simply wasn't
authorized on `frit`. The user granted it; a fresh commit on PR #156
then produced `codecov/patch` ("Coverage not affected") and
`codecov/project` ("88.21% (+0.00%) compared to e90c2d6"), both
`completed`/`success` via the Checks API.

**Gate.** All four items:

1. `codecov.yml` validates against `https://codecov.io/validate`.
2. `go test ./...`, `golangci-lint`, `mdsmith check .` stay green.
3. `codecov/project` and `codecov/patch` posted `success` on PR #156
   with a real baseline (`88.21% (+0.00%) compared to e90c2d6`), not
   `if_not_found: success` firing for lack of one.
4. `main`'s `required_status_checks.contexts` now includes
   `codecov/project` and `codecov/patch`, confirmed with the user
   before the change — [PR #156](https://github.com/jeduden/frit/pull/156)
   shows both as required, passing checks.

**Plan closed.** Every Acceptance Criterion in [plan.md](plan.md) is
met. PR #156 carries only plan/handoff documentation — no product
code beyond phase 1's, already merged — and is ready to merge.
