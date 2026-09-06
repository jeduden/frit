#!/usr/bin/env bash
# Fails when any of the named packages drops below 100% line coverage.
#
# go test -cover measures line coverage per package but never gates on
# it; this is the wrapper each line-coverage phase adds once its
# package reaches 100%, so a later change that drops a line back below
# it reddens CI instead of drifting unnoticed. Called with the package
# paths to gate, e.g. `scripts/check-coverage.sh ./internal/report`.
set -euo pipefail

status=0
for pkg in "$@"; do
  # `set -e` would otherwise abort the whole script right here the
  # moment go test exits non-zero — on a compile error or a failing
  # test, not just low coverage — printing nothing and never reaching
  # the remaining packages. The `if !` guard keeps that failure local
  # to this iteration.
  if ! out=$(go test "$pkg" -cover 2>&1); then
    echo "$out"
    echo "check-coverage: go test failed for $pkg" >&2
    status=1
    continue
  fi
  echo "$out"
  # Likewise, `pipefail` makes a no-match grep (a package with no test
  # files has no "coverage:" line at all) fail the pipeline; `|| true`
  # keeps that reaching the explicit -z check below instead of exiting
  # the script outright.
  pct=$( { echo "$out" | grep -oE 'coverage: [0-9]+\.[0-9]+% of statements' \
    | grep -oE '[0-9]+\.[0-9]+'; } || true)
  if [ -z "$pct" ]; then
    echo "check-coverage: could not read coverage for $pkg" >&2
    status=1
    continue
  fi
  if [ "$pct" != "100.0" ]; then
    echo "check-coverage: $pkg is at $pct%, want 100.0%" >&2
    status=1
  fi
done

exit "$status"
