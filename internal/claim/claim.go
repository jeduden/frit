// Package claim mints frit's holds. A hold on a plan is a git ref, and
// the claim is leased by pushing an empty marker commit to a shared
// remote with --force-with-lease against a ref that must not yet exist.
// The remote's server-side check is the arbitration: two machines
// racing for the same plan resolve to exactly one winner.
//
// This is frit's first git write. Everything else in frit indexes and
// displays; a lease is the one mutation frit owns, because a hold has
// to be atomic and a ref push is the only atom git offers.
package claim

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// ErrLostRace reports a claim that lost the push: another machine
// already holds the ref on the remote. It is a sentinel so a caller can
// tell the one expected, non-fatal outcome — someone got there first —
// apart from a git fault, and report it rather than crash on it.
var ErrLostRace = errors.New("lost the race")

// Branch is the hold branch a plan is claimed on: plan/<id>, id only.
// Nothing derived from local state — a file name, a slug, a title —
// reaches the ref, so two machines can never name the same plan
// differently and a renamed plan file cannot mint a second hold. It is
// derived, never stored, and the default hold patterns read it back —
// a lease frit writes is a hold frit finds.
func Branch(planID int64) string {
	return leaseBranch(planID)
}

// remoteHolder returns the commit the hold ref points at on the remote,
// or "" when the ref is absent or the remote cannot be read. It reads
// `ls-remote`'s stable `<sha>\t<ref>` plumbing output — not the push's
// human-readable stderr, which the project rule forbids parsing — so a
// failed push can be classified by what actually holds the ref:
//
//   - our own marker   the push landed; the claim is ours
//   - a different sha   another machine won the race
//   - "" (none)         the push left no ref; a real fault
//
// Reading "" on an unreadable remote folds the unconfirmable case into a
// real fault, which fails safe: a retry re-attempts the push cleanly.
// A caller for whom "gone" and "unreadable" must not fold together — a
// scavenge deciding whether a ref is safely absent — uses
// remoteHolderErr instead.
func remoteHolder(repoDir, remote, ref string, run gitwt.Runner) string {
	sha, _ := remoteHolderErr(repoDir, remote, ref, run)

	return sha
}

// remoteHolderErr is remoteHolder with the read fault kept apart from
// the absent ref: ("", nil) is a remote that answered and has no such
// ref, while an error is a remote that could not be read at all.
func remoteHolderErr(
	repoDir, remote, ref string, run gitwt.Runner,
) (string, error) {
	// No --heads filter: callers pass a full ref name, and the lease
	// path also reads refs outside refs/heads — the rescue refs.
	out, err := run(repoDir, "ls-remote", remote, ref)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}

	return fields[0], nil
}

// MarkerHost reads the host recorded in the claim marker for a plan on a
// branch, or "" when no frit marker is reachable — a non-frit branch, or
// objects not yet in the local store.
//
// It reuses the marker the lease wrote, so a preflight can tell a foreign
// checkout from an own one without a second parser: standing in a shared
// clone on a branch another host holds must not read as this session's
// own lane. An unreadable marker returning "" fails open — the guard
// then does not refuse, which is the safe default for a non-frit branch.
func MarkerHost(
	repoDir, branch string, planID int64, run gitwt.Runner,
) string {
	body, ok := holderMarker(repoDir, planID, branch, run)
	if !ok {
		return ""
	}

	return markerHost(body)
}

// holderMarker returns the claim marker's body from the holder branch.
//
// The host line lives on the marker commit frit minted, not the branch
// tip: an active lane advances its branch past the marker with real work,
// so the tip carries no host line. The marker is found by the stable
// subject frit writes — `plan <id>: claim` — even when work commits sit on
// top of it. ok is false when no such marker is reachable — a non-frit
// branch on the hold ref, or objects not yet in the local store.
func holderMarker(
	repoDir string, planID int64, tip string, run gitwt.Runner,
) (string, bool) {
	// No trailing space after "claim": the legacy subject carries a lane
	// slug behind it, the lease subject ends there, and both must match.
	pattern := fmt.Sprintf("^plan %d: claim", planID)
	body, err := trimmed(run(repoDir, "log", "-1",
		"--grep="+pattern, "--format=%B", tip))
	if err != nil || body == "" {
		return "", false
	}

	return body, true
}

// markerHost reads the holding machine from a claim marker's body, or
// "" when there is none — a plain work commit that landed carries no
// such line, and that absence is not a fault. Legacy markers record it
// as host:, lease markers as holder:; both read as the host.
func markerHost(body string) string {
	for _, line := range strings.Split(body, "\n") {
		for _, key := range []string{"host:", "holder:"} {
			if rest, ok := strings.CutPrefix(
				strings.TrimSpace(line), key); ok {
				return strings.TrimSpace(rest)
			}
		}
	}

	return ""
}

// landedTip is the landed check shared with the lease path.
func landedTip(repoDir, baseRef, remote, tip string, run gitwt.Runner) bool {
	return isAncestor(repoDir, tip, freshBase(repoDir, baseRef, remote, run), run)
}

// freshBase resolves the base to judge evidence against, refreshed
// from the remote when it can be — FETCH_HEAD after a fetch of the
// base branch — and the local view otherwise, so a stale origin/main
// does not decide what has landed.
func freshBase(repoDir, baseRef, remote string, run gitwt.Runner) string {
	if branch := baseBranch(baseRef, remote); branch != "" {
		if _, err := run(repoDir, "fetch", "--quiet",
			remote, branch); err == nil {
			return "FETCH_HEAD"
		}
	}

	return baseRef
}

// baseBranch reduces a base ref to the remote branch name to fetch for a
// fresh landed check: refs/remotes/<remote>/main, <remote>/main and
// refs/heads/main all reduce to main. A base with no such prefix is
// returned unchanged, so a fetch of it either refreshes the base or fails
// into the local fallback.
func baseBranch(base, remote string) string {
	base = strings.TrimPrefix(base, "refs/remotes/")
	base = strings.TrimPrefix(base, "refs/heads/")

	return strings.TrimPrefix(base, remote+"/")
}

// isAncestor reports whether sha is already merged into base. A non-zero
// exit — the not-an-ancestor answer, or any git fault — reads as false,
// which is the safe default: a claim is called landed only on a clear yes.
func isAncestor(dir, sha, base string, run gitwt.Runner) bool {
	_, err := run(dir, "merge-base", "--is-ancestor", sha, base)

	return err == nil
}

// trimmed drops the trailing newline git prints after a hash, passing an
// error through untouched so callers can bail on the first failure.
func trimmed(out []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(out), "\n"), nil
}
