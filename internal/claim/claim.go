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

// Branch is the hold branch a plan is claimed on: plan/<id>-<slug>,
// with the slug taken from the plan file name after its id prefix. It
// is derived, never stored, so it is the same name the default hold
// pattern reads back — a lease frit writes is a hold frit finds. A file
// whose name carries no `<id>_` prefix contributes its whole stem, so
// the branch is always well formed.
func Branch(planID int64, planPath string) string {
	stem := strings.TrimSuffix(filepath.Base(planPath), ".md")
	slug := stem
	if i := strings.IndexByte(stem, '_'); i >= 0 {
		slug = stem[i+1:]
	}

	return fmt.Sprintf("plan/%d-%s", planID, slug)
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
		// Roll the local ref back even though we are returning an error,
		// so the next attempt does not trip over a stale claim.
		_, _ = run(repoDir, "update-ref", "-d", ref)

		return Result{}, fmt.Errorf(
			"%w for plan %d: the claim ref already exists",
			ErrLostRace, opts.PlanID)
	}

	return Result{Branch: opts.Branch, BaseSHA: baseSHA}, nil
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
