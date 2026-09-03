---
id: 2609022208
title: Coverage uploads to Codecov and can only improve
status: "🔲"
summary: >-
  CI already measures coverage — every job builds one `cover.out` and
  parks it as an artifact — but the number lives and dies inside the
  run: nobody sees a trend, and nothing stops a change from quietly
  eroding it. This plan uploads that profile to Codecov the tokenless
  way, using the GitHub Actions OIDC identity rather than a stored
  upload token, the same wiring mdsmith uses. Then it turns the number
  into a gate: a root `codecov.yml` whose project status may dip at
  most 0.5% before the required check fails, so coverage can drift down
  by noise but never fall for real, and new code carries its own tests.
model: sonnet
depends-on: []
---
# Coverage uploads to Codecov and can only improve

## Goal

Every push and same-repo pull request uploads its Go coverage profile
to Codecov over the GitHub Actions OIDC identity — no stored token —
and Codecov posts a required status check that fails when project
coverage drops by more than 0.5%. Coverage can only improve, give or
take the noise floor.

## Context

**What CI already does.** The `test` job in
[ci.yml](../../.github/workflows/ci.yml)
runs `go test ./... -coverpkg=./... -coverprofile=cover.out`,
summarises the total with `go tool cover -func`, and uploads
`cover.out` as an artifact. So the measurement exists and is trusted;
what is missing is a destination that keeps the history and a gate that
reads it. Nothing here changes how coverage is measured.

**The OIDC way, not a token.** mdsmith uploads with
`codecov/codecov-action` configured `use_oidc: true`: the action
exchanges the job's GitHub OIDC identity for a Codecov upload
credential at run time, so no `CODECOV_TOKEN` secret is stored or
exposed. That needs `id-token: write` on the uploading job — the
top-level `permissions:` stays `contents: read`, and only the test job
is granted the token scope. The repository's Codecov side is already
configured to trust that identity.

**Fork PRs cannot mint the identity.** A pull request from a fork runs
with no `id-token` access, so the upload step is guarded — mdsmith's
own `ci.yml` gates it on `github.event_name
!= 'pull_request' || …head.repo.full_name == github.repository`. The
status gate then declares `if_no_uploads: success` and `if_not_found:
success` so a fork PR's required check still posts green rather than
hanging, while a same-repo PR gates on real numbers.

**The gate is the point.** The user's rule — coverage can only improve,
with a 0.5% tolerance — is Codecov's `coverage.status.project` with
`target: auto` (the base commit's coverage) and `threshold: 0.5%`. A
drop within 0.5% passes, absorbing the drift that deleting
well-covered code causes; a larger drop fails the check. A `patch`
status holds new code to its own coverage so the project average cannot
be gamed by adding untested lines beside well-covered ones.

**Reuse first, and one lesson borrowed.** The action is pinned by the
same SHA mdsmith already vets, so zizmor's impostor-commit audit —
already a CI job here — covers it. frit is a single-language Go tree
with no in-tree vendored code, so the plan needs none of mdsmith's
`ignore:`, flag or component stanzas; `codecov.yml` stays small. The
one lesson carried over is `codecov: notify: wait_for_ci: true`, which
mdsmith added after the status check raced ahead of the upload and
graded a stale cache.

## Tasks

1. Phase 1 (proving slice): add the Codecov upload to the test job in
   [.github/workflows/ci.yml](../../.github/workflows/ci.yml) over OIDC
   — `id-token: write` on that job, the action pinned by SHA,
   `use_oidc: true`, the fork-PR guard — and a root `codecov.yml` that
   validates against Codecov's schema. Prove the profile reaches
   Codecov on a real run.
2. Later, once uploads are confirmed flowing: add the
   `coverage.status.project` gate at `threshold: 0.5%` and a `patch`
   gate to `codecov.yml`, with `if_no_uploads`/`if_not_found: success`
   for fork PRs, and make the Codecov project check a required status
   check on `main`.

## Execution

| Phase | Title                                | Tier   | Gate                                                                                                           |
| ----- | ------------------------------------ | ------ | -------------------------------------------------------------------------------------------------------------- |
| 1     | Coverage uploads to Codecov via OIDC | sonnet | `codecov.yml` validates via `https://codecov.io/validate`; zizmor clean on the edited workflow; a push uploads |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                              |
| --- | ------ | -------------------------------------------------- |
| 1   | 🔲     | [Coverage uploads to Codecov via OIDC](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] The test job uploads `cover.out` to Codecov using
      `codecov/codecov-action` with `use_oidc: true` and no stored
      token, and the job is granted `id-token: write`
- [ ] The upload step is skipped on fork pull requests and runs on
      pushes and same-repo pull requests
- [ ] `codecov.yml` validates against Codecov's config validator
- [ ] `zizmor` reports no findings on the edited workflow
- [ ] Codecov posts a `project` status that fails when coverage drops
      more than 0.5%, and a `patch` status on new code, both posting
      green on fork PRs
- [ ] The Codecov project check is a required status check on `main`
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
