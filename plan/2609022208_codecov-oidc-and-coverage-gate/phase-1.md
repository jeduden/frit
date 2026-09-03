---
n: 1
title: Coverage uploads to Codecov via OIDC
status: "🔲"
result: false
---
Upload the coverage profile CI already builds to Codecov, using the
GitHub Actions OIDC identity rather than a stored token. Add a root
`codecov.yml` that Codecov accepts, and a Codecov badge to the README.
This phase proves the profile reaches Codecov end to end; the coverage
gate itself is phase 2.

**Assumes.** [.github/workflows/ci.yml](../../.github/workflows/ci.yml)
already runs `go test ./... -coverpkg=./... -coverprofile=cover.out` in
the `test` job and keeps that file for the rest of the job. The
top-level `permissions:` is `contents: read`. A `workflows` job runs
zizmor over every workflow and fails on any finding, including an
unpinned action. mdsmith's own `ci.yml` uploads with
`codecov/codecov-action@57e3a136b779b570ffcdbf80b3bdc90e7fab3de2`
(v6.0.0), `use_oidc: true`, and a fork-PR guard — the working shape to
copy.

**RED / GREEN — the workflow.** A YAML edit has no Go test; its red is
the checks the change must satisfy, run before and after.

- Grant the `test` job `id-token: write` alongside `contents: read`, so
  only that job can mint the OIDC identity. Leave the top-level
  permission at `contents: read`.
- After the `Summarise coverage` step, add an `Upload coverage to
  Codecov` step: `uses: codecov/codecov-action` pinned by the SHA
  above, `with: { use_oidc: true, files: cover.out,
  fail_ci_if_error: false }`. Guard it `if: github.event_name !=
  'pull_request' || github.event.pull_request.head.repo.full_name ==
  github.repository`, so a fork PR — which has no `id-token` — skips it.
- Write `codecov.yml` at the repository root. For this phase it need
  only carry `codecov: notify: wait_for_ci: true` and a `comment:`
  block; the `coverage.status` gate is phase 2. Keep it minimal — frit
  is a single-language Go tree with no vendored code, so no `ignore:`,
  flag or component stanzas.
- Add a Codecov badge to [README.md](../../README.md), on its own line
  directly under the `# frit` title: a linked
  `https://codecov.io/gh/jeduden/frit/graph/badge.svg` image pointing
  at the project page, in the markdown Codecov's settings page gives.
  Confirm the exact owner/repo slug against the Codecov project rather
  than assuming it.

**Gate.** Three checks, none of which needs the number to move:

1. `codecov.yml` validates: `curl -X POST --data-binary @codecov.yml
   https://codecov.io/validate` returns the parsed config, not an
   error. This is the phase's own claim-gate — the config is accepted
   by the service that will read it.
2. zizmor is clean on the edited workflow, matching what the CI
   `workflows` job enforces: run the same pinned zizmor over
   `.github/workflows/ci.yml` and confirm no finding — the added action
   is pinned by SHA and leaves no token in the checkout.
3. On a real CI run of this branch — its same-repo pull request, or a
   push to `main` — the `test` job's upload step runs and the commit
   appears on Codecov with a coverage report. A fork PR would show the
   step skipped. (`on.push` is `branches: [main]`, so a feature-branch
   push alone triggers no run.)

Then the full `go test ./...` and lint stay green — this change touches
no Go — and `mdsmith check .` passes on the plan. `codecov.yml` is
YAML, not markdown, so mdsmith does not lint it; gate 1 validates it
against Codecov instead.
