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
	"path/filepath"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// ErrLostRace reports a claim that lost the push: another machine
// already holds the ref on the remote. It is a sentinel so a caller can
// tell the one expected, non-fatal outcome — someone got there first —
// apart from a git fault, and report it rather than crash on it.
var ErrLostRace = errors.New("lost the race")

// Holder describes who holds the ref that won a lost push, read from its
// marker. It turns the refusal's guess into a fact: a claim held on this
// host reads differently from one held elsewhere, and a ref already
// merged into the base is landed work with a stale status, not a
// competitor. Known is false when the marker could not be read, so the
// caller falls back to the original wording rather than misreport.
type Holder struct {
	Host     string // the host recorded in the holder's marker; "" if none
	ThisHost bool   // the holder's host is this run's host
	Landed   bool   // the holder's ref is merged into the base
	Known    bool   // the marker was read; false → fall back to old wording
}

// LostRaceError reports a claim that lost the push and carries who holds
// the ref. It wraps ErrLostRace, so a caller can still test the sentinel
// with errors.Is while reading the Holder with errors.As.
type LostRaceError struct {
	PlanID int64
	Holder Holder
}

func (e *LostRaceError) Error() string {
	return fmt.Sprintf(
		"lost the race for plan %d: the claim ref already exists", e.PlanID)
}

func (e *LostRaceError) Unwrap() error { return ErrLostRace }

// Options describes the lease to mint.
type Options struct {
	Branch   string // the hold branch to mint, e.g. "plan/7-shader-unit"
	Base     string // the ref the lease is dated against, e.g. "origin/main"
	Remote   string // e.g. "origin"
	PlanID   int64
	PlanFile string // repo-relative plan file path, recorded in the marker
	Lane     string // worktree path the claim is for; may be "" (recorded in the marker)
	Host     string // machine name recorded in the marker
}

// Result is the minted lease: the branch that now holds the plan and the
// base commit it was dated against.
type Result struct {
	Branch  string
	BaseSHA string
}

// Branch is the hold branch a plan is claimed on: plan/<id>, id only.
// Nothing derived from local state — a file name, a slug, a title —
// reaches the ref, so two machines can never name the same plan
// differently and a renamed plan file cannot mint a second hold. It is
// derived, never stored, and the default hold patterns read it back —
// a lease frit writes is a hold frit finds.
func Branch(planID int64) string {
	return leaseBranch(planID)
}

// Mint leases the hold branch for a plan.
//
// The marker is an empty commit — the same tree as Base — so the claim
// touches no file; it exists only to carry a ref. The push uses
// --force-with-lease with an empty expected value, which requires the
// ref to be absent on the remote: that is the atomic arbitration. When
// the push loses, the local ref is rolled back before the error returns
// so a retry starts from a clean state.
func Mint(repoDir string, opts Options, run gitwt.Runner) (Result, error) {
	baseSHA, err := trimmed(run(repoDir, "rev-parse", opts.Base))
	if err != nil {
		return Result{}, err
	}

	tree, err := trimmed(run(repoDir, "rev-parse", opts.Base+"^{tree}"))
	if err != nil {
		return Result{}, err
	}

	marker, err := trimmed(run(repoDir, "commit-tree", tree,
		"-p", baseSHA, "-m", markerMessage(opts, baseSHA)))
	if err != nil {
		return Result{}, err
	}

	ref := "refs/heads/" + opts.Branch
	if _, err := run(repoDir, "update-ref", ref, marker); err != nil {
		return Result{}, err
	}

	// The empty expected value after the colon means the remote ref must
	// not already exist; the server rejects the push if it does.
	if _, err := run(repoDir, "push",
		"--force-with-lease="+ref+":",
		opts.Remote, marker+":"+ref); err != nil {
		// A failed push has three outcomes, told apart by what commit holds
		// the ref on the remote — never by git's stderr, which the project
		// rule forbids parsing and which a hook can fill with misleading
		// wording like "already exists".
		holderSHA := remoteHolder(repoDir, opts.Remote, ref, run)
		switch holderSHA {
		case marker:
			// Our own marker is on the remote: the push landed even though
			// the client reported an error, e.g. a connection dropped after
			// the ref transaction committed. The claim is ours, so keep the
			// local ref and report success rather than orphan it as a race
			// lost to a machine that is really us.
			return Result{Branch: opts.Branch, BaseSHA: baseSHA}, nil
		case "":
			// Nothing on the remote, or it could not be read: the push left
			// no ref, so this is a real fault. Roll the local ref back so a
			// retry starts clean.
			_, _ = run(repoDir, "update-ref", "-d", ref)

			return Result{}, fmt.Errorf(
				"push claim for plan %d: %w", opts.PlanID, err)
		default:
			// A different commit holds the ref: another machine won, or our
			// own branch already landed with a status never set to ✅. Roll
			// the local ref back, then read the holder's marker so the
			// refusal can name who really holds the plan rather than always
			// blaming a machine that may not be there.
			_, _ = run(repoDir, "update-ref", "-d", ref)

			return Result{}, &LostRaceError{
				PlanID: opts.PlanID,
				Holder: readHolder(repoDir, opts, holderSHA, run),
			}
		}
	}

	return Result{Branch: opts.Branch, BaseSHA: baseSHA}, nil
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

// readHolder names who holds the ref that won a lost push. It reads the
// holder's marker for the host that took the claim, and asks whether the
// holder's work is already merged into the base — landed work whose status
// was never set to ✅. Both are on the already-slow failure path, so
// naming a holder costs nothing on the winning push.
//
// An unreadable marker yields the zero Holder, whose Known is false, so
// the caller falls back to the original wording rather than misreport.
func readHolder(
	repoDir string, opts Options, tip string, run gitwt.Runner,
) Holder {
	body, ok := holderMarker(repoDir, opts, tip, run)
	if !ok {
		// The holder's commits may not be local — a claim from another
		// machine was never fetched. Bring the branch in once, then retry
		// before falling back to the original wording.
		if _, err := run(repoDir, "fetch", "--quiet",
			opts.Remote, "refs/heads/"+opts.Branch); err != nil {
			return Holder{}
		}
		if body, ok = holderMarker(repoDir, opts, tip, run); !ok {
			return Holder{}
		}
	}
	host := markerHost(body)

	return Holder{
		Host:     host,
		ThisHost: host != "" && host == opts.Host,
		Landed:   landed(repoDir, opts, tip, run),
		Known:    true,
	}
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
	body, ok := holderMarker(repoDir, Options{PlanID: planID}, branch, run)
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
	repoDir string, opts Options, tip string, run gitwt.Runner,
) (string, bool) {
	// No trailing space after "claim": the legacy subject carries a lane
	// slug behind it, the lease subject ends there, and both must match.
	pattern := fmt.Sprintf("^plan %d: claim", opts.PlanID)
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

// landed reports whether the holder's work has reached the default branch
// on the remote — merged work whose branch was left behind with a status
// never set to ✅. It refreshes the base from the remote first, so a stale
// local view of the default branch does not read a claim merged on another
// machine as a live competitor. A base that cannot be fetched falls back
// to the local view rather than failing the classification.
func landed(repoDir string, opts Options, tip string, run gitwt.Runner) bool {
	return landedTip(repoDir, opts.Base, opts.Remote, tip, run)
}

// landedTip is the landed check itself, shared with the lease path,
// which carries its base and remote outside an Options.
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

// markerMessage builds the lease's commit body.
//
// The body records what the claim is for so the ref alone tells the full
// story: the lane it holds, the host that took it, the base it was dated
// against and the plan file it belongs to. An empty Lane records "-" for
// both the title and the lane line rather than filepath.Base's ".".
func markerMessage(opts Options, baseSHA string) string {
	lane := opts.Lane
	base := filepath.Base(lane)
	if lane == "" {
		lane = "-"
		base = "-"
	}

	return fmt.Sprintf(
		"plan %d: claim %s\n\n"+
			"lane:     %s\n"+
			"host:     %s\n"+
			"base:     %s\n"+
			"plan:     %s",
		opts.PlanID, base, lane, opts.Host, baseSHA, opts.PlanFile)
}

// trimmed drops the trailing newline git prints after a hash, passing an
// error through untouched so callers can bail on the first failure.
func trimmed(out []byte, err error) (string, error) {
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(out), "\n"), nil
}
