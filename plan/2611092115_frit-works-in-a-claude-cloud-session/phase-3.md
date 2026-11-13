---
n: 3
title: the review's design decisions
status: "✅"
result: false
---
A code review of phases 1 and 2 raised seven questions. The owner
decided each one:

1. **Where a cloud session's PR comes from.** From `plan/<id>`
   itself. Merging it lands the lease, and GitHub's auto-delete
   removes the branch, so the session never deletes. The
   `plan-phase` skill drops the copy to the session's own branch.
   S55 already covers a merge that deletes the branch. C21's copy
   flow still lands, so C21 stays.
2. **Auto-delete on this repository.** The owner turns it on by
   hand. A cloud session cannot write repository settings: the
   proxy answers 403.
3. **doctor checks it.** For a GitHub origin, `frit doctor` asks
   through `gh`, and warns when the setting is off. It notes the
   check as skipped where `gh` is missing or fails. frit never
   changes the setting. A non-GitHub origin is not asked, and gets
   no note: a note every run would be noise.
4. **holder in a committed `.frit.yml`.** It is ignored there, since
   every clone shares the file, and `frit doctor` reports it. The
   flag, `FRIT_HOLDER`, `$FRIT_CONFIG` and the user config still set
   it.
5. **The rescue name.** `frit-rescue/<id>/...`. A branch named
   exactly `frit` would have made git refuse every park under
   `frit/rescue/`, which never shipped.
6. **Leases minted as `vm` before upgrading.** Left to the takeover
   window. No code.
7. **Fetching rescue branches.** frit's own fetch carries the
   remote's configured refspecs on its command line, plus a negative
   one for `frit-rescue/`. Git config is untouched, so a plain
   `git fetch` still downloads them.

BDD coverage: C27 now pins the `frit-rescue/` name. The rest is
covered by unit tests: a real-git fetch, the config loader, and
doctor run against a fake `gh`. None is a lease race.

Gate: `go test ./...`, the coverage gate, golangci-lint and
`mdsmith check .` pass.
