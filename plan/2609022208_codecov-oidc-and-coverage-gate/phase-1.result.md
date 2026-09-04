---
n: 1
title: Coverage uploads to Codecov via OIDC
status: "✅"
result: true
summary: >-
  The `test` job now uploads `cover.out` to Codecov over OIDC — no
  stored token — guarded off on fork PRs; `codecov.yml` validates and
  the README carries the project's badge.
---
## Handoff

**Done.** The `test` job in
[ci.yml](../../.github/workflows/ci.yml) is granted `id-token: write`
alongside `contents: read` (top-level `permissions:` stays
`contents: read`), and after `Summarise coverage` an `Upload coverage
to Codecov` step runs `codecov/codecov-action`, pinned to
`57e3a136b779b570ffcdbf80b3bdc90e7fab3de2` (v6.0.0), with
`use_oidc: true`, `files: cover.out`, `fail_ci_if_error: false`,
guarded on `github.event_name != 'pull_request' ||
github.event.pull_request.head.repo.full_name == github.repository` —
the same shape mdsmith's own `ci.yml` runs. A root `codecov.yml`
carries `codecov: notify: wait_for_ci: true` and a `comment:` layout
only; no `coverage.status` gate yet — that is phase 2. The README
gained a Codecov badge (`[![codecov][codecov-badge]][codecov-project]`)
directly under the `# frit` title, with the two link references at the
bottom of the file — a bare inline link ran the line past mdsmith's
80-column `MDS001` limit.

**Gate.** Two of the three checks ran and passed locally:

1. `curl -X POST --data-binary @codecov.yml https://codecov.io/validate`
   returned the parsed config — `Valid!`.
2. zizmor v1.29.0 (`docker run ghcr.io/zizmorcore/zizmor:1.29.0`, the
   digest the CI action itself pins) reported no findings on the
   edited `.github/workflows/ci.yml`, run offline (no `GITHUB_TOKEN`
   locally, so the token-gated audits did not run — CI's `workflows`
   job runs the same version with a token and is the check that
   actually gates the PR).
3. **Not verifiable from this session** — needs a real push to `main`
   or a same-repo PR to confirm the upload step runs and the commit
   appears on Codecov with a coverage report. Watch the first CI run
   on this branch's PR for the `Upload coverage to Codecov` step and
   check <https://codecov.io/gh/jeduden/frit> for the commit.

`go build ./...` and `go vet ./...` stay green (no Go touched);
`mdsmith check .` passes on the full tree (245 checked, 0 failures).

**Review fix.** `/code-review high --fix` caught the upload step
skipping on a failed `go test` — its plain `if:` got an implicit
`success()` ANDed in, unlike `Summarise coverage` and `upload-artifact`
just above it, which both carry `!cancelled()` for exactly the reason
this file states in its own comment: coverage from a failing run is
the one most wanted. Fixed by adding `!cancelled() &&` to the step's
condition. The review also flagged that `codecov-action` detects fork
PRs internally and could upload tokenlessly instead of skipping
outright — left as-is: reversing the deliberate fork guard is a policy
call for phase 2, not a bug fix.

**Inherits into phase 2.** Uploads are wired but unconfirmed on a real
run — phase 2 should start by watching that first CI run land on
Codecov before adding the `coverage.status.project`/`patch` gate at
`threshold: 0.5%`, `if_no_uploads`/`if_not_found: success`, and making
the Codecov check required on `main`.
