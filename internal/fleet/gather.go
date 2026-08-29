package fleet

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/headroom"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
	"github.com/jeduden/frit/internal/repocfg"
)

// Problem is one repository, or one file within it, frit could not
// read. It travels with the result rather than ending the gather, so a
// single broken checkout does not blind the whole board.
//
// NotPlan marks the benign kind: a markdown file in the plan directory
// that carries no front matter is simply not a plan, not a fault. A
// repository keeps a PLAN.md index and old notes beside its plans, so
// these are common and drowning the real failures. They are hidden
// unless a caller asks for everything.
type Problem struct {
	Repo    string
	Err     error
	NotPlan bool
}

// Coord is the per-repository state a lease is minted from: where the
// repository lives, the remote a claim is pushed to, and the base a
// lease is dated against. The gather already resolves all three walking
// the fleet, so a mutating verb reads them off the result rather than
// walking the whole fleet a second time to re-derive them.
//
// The base is the config's when set, otherwise the default-ref cascade
// the gather already runs for its merged-ref filter — the same cascade
// a lease dates against.
type Coord struct {
	Path   string
	Remote string
	Base   string
	// TakeoverWindow and SampleGap are the repository's own staleness
	// clock (T, S_max), read from .frit.yml with the discovery
	// defaults when it declares neither (F12).
	TakeoverWindow time.Duration
	SampleGap      time.Duration
}

// Result is a gathered fleet: every plan's authoritative version across
// every repository and ref, the coordinate a lease is minted from for
// each repository, and the problems met on the way.
//
// The coordinates are keyed by repository name — the same name a plan
// carries — and are per repository, not per plan, so they travel beside
// the plans rather than copied onto each one a repository holds.
type Result struct {
	Plans    []discovery.Plan
	Problems []Problem
	Coords   map[string]Coord
}

// Options tunes how Gather reads a repository.
type Options struct {
	// Fetch refreshes each repository's remote-tracking refs before
	// Gather reads landed evidence off them, so work merged and its
	// lease branch deleted on the remote reads as landed rather than
	// held on a checkout that has not fetched since.
	Fetch bool
	// Headroom runs the internal/headroom oracle and opens the
	// mdsmith session it needs. Only the verbs that render the
	// signal — ready and pick — ask for it; the other thirteen never
	// pay for a session open or an oracle pass they would discard.
	Headroom bool
}

// Gather reads every repository under root and flattens its plan index
// into the view discovery works over.
//
// It is the plans walk plus holds: the same cross-ref index frit plans
// builds, joined to the claims frit orphans reads, so a plan arrives
// carrying both its dependency edges and whether a lane already holds
// it. One unreadable repository is recorded and stepped over; a bad
// front matter file within a good repository is recorded too.
func Gather(
	root, host string, run gitwt.Runner, pipe gitwt.PipeRunner, opts Options,
) (Result, error) {
	repos, skipped, err := discover.Repos(root, run)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Plans:    []discovery.Plan{},
		Problems: []Problem{},
		Coords:   map[string]Coord{},
	}
	for _, s := range skipped {
		res.Problems = append(res.Problems, Problem{
			Repo: filepath.Base(s.Dir),
			Err: fmt.Errorf(
				"could not read repository at %s: %w", s.Dir, s.Err),
		})
	}
	ambiguous := map[string]bool{}
	for _, repo := range repos {
		entries, held, leaseTips, coord, headrooms, problems, err := gatherRepo(
			host, repo, run, pipe, opts)
		if err != nil {
			res.Problems = append(res.Problems,
				Problem{Repo: repo.Name, Err: err})
			continue
		}
		res.Problems = append(res.Problems, problems...)
		recordCoord(&res, ambiguous, repo.Name, coord)
		for _, e := range entries {
			res.Plans = append(res.Plans,
				planOf(repo.Name, e, held, leaseTips, headrooms))
		}
	}

	return res, nil
}

// recordCoord files a repository's lease coordinate under its name,
// unless another repository under the root already claimed that name.
//
// The fleet keys every plan by its repository's basename, so two
// checkouts sharing one cannot be told apart — a coordinate under that
// name could mint a lease into the wrong repository. When two collide
// the coordinate is dropped and the collision recorded as a problem, so
// a mutating verb refuses rather than guesses. The plans of both repos
// are still gathered, so the read-only board stays useful; only the
// lease, which must land in one exact checkout, is withheld.
func recordCoord(
	res *Result, ambiguous map[string]bool, name string, coord Coord,
) {
	if ambiguous[name] {
		return
	}
	if _, dup := res.Coords[name]; dup {
		delete(res.Coords, name)
		ambiguous[name] = true
		res.Problems = append(res.Problems, Problem{
			Repo: name,
			Err: fmt.Errorf("repository name %q is not unique under the "+
				"root; rename one so a claim lands in the right checkout",
				name),
		})
		return
	}
	res.Coords[name] = coord
}

// parseProblems reports index.Build's parse errors and plans.Collect's
// mislaid files as one repository's problems, split out of gatherRepo
// to keep it under the linter's length cap.
func parseProblems(repoName string, errs []error, mislaid []string) []Problem {
	problems := make([]Problem, 0, len(errs)+len(mislaid))
	for _, e := range errs {
		problems = append(problems, Problem{
			Repo:    repoName,
			Err:     e,
			NotPlan: errors.Is(e, planmeta.ErrNoFrontMatter),
		})
	}
	for _, p := range mislaid {
		problems = append(problems, Problem{
			Repo: repoName,
			Err: fmt.Errorf(
				"%s looks like a plan but is not %s; it is not read",
				p, plans.FixedName),
		})
	}

	return problems
}

// gatherRepo reads one repository's plans, the ids its lanes hold, and
// the coordinate a lease is minted from. The coordinate reuses the
// config and the default-ref cascade this loop already resolved, so it
// costs no new walk or subprocess.
func gatherRepo(
	host string, repo discover.Repo,
	run gitwt.Runner, pipe gitwt.PipeRunner, opts Options,
) ([]index.Entry, map[int64][]string, map[int64]string, Coord,
	map[int64]int, []Problem, error,
) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return nil, nil, nil, Coord{}, nil, nil, err
	}

	var fetchErr error
	if opts.Fetch {
		fetchErr = fetchRemote(repo.Path, cfg.Remote, run)
	}

	files, mislaid, err := plans.Collect(repo.Path, cfg.PlanDir, run, pipe)
	if err != nil {
		return nil, nil, nil, Coord{}, nil, nil, err
	}

	preferred := gitobj.DefaultRef(repo.Path, run)
	entries, errs := index.Build(host, repo.Name, preferred, files)
	problems := parseProblems(repo.Name, errs, mislaid)

	refs, err := gitobj.Refs(repo.Path, run)
	if err != nil {
		return nil, nil, nil, Coord{}, nil, nil, err
	}
	if p := staleFetch(repo, cfg.Remote, fetchErr, refs); p != nil {
		problems = append(problems, *p)
	}
	if p := laggingDefaultBranch(repo, cfg.Remote, preferred, refs, run); p != nil {
		problems = append(problems, *p)
	}

	held, leaseTips, err := heldBranches(
		repo, cfg, preferred, refs, index.LandedIDs(entries, preferred), run)
	if err != nil {
		return nil, nil, nil, Coord{}, nil, nil, err
	}

	var headrooms map[int64]int
	if opts.Headroom {
		var problem *Problem
		headrooms, problem = headroomFor(repo, cfg, files, entries)
		if problem != nil {
			problems = append(problems, *problem)
		}
	}

	return entries, held, leaseTips, coordOf(repo, cfg, preferred),
		headrooms, problems, nil
}

// headroomFor computes each entry's headroom shortfall against the
// repository's own reserve, using the same internal/headroom oracle
// doctor runs. gatherRepo calls it only when Options.Headroom asks for
// it — ready and pick are the only two of the fleet's fifteen verbs
// that render the signal, so the other thirteen never pay for the
// session it opens. A reserve of 0 disables it outright too — no
// session is even opened. A session that fails to open (a malformed
// .mdsmith.yml) is reported as a problem rather than failing the whole
// gather: the fleet index carries no schema requirement of its own the
// way doctor does, so one repository's broken config must not blind
// every other plan. A plan with room enough is simply absent from the
// map; a caller reads a missing entry as 0, the same as a plan with
// room.
func headroomFor(
	repo discover.Repo, cfg repocfg.Config, files []plans.File, entries []index.Entry,
) (map[int64]int, *Problem) {
	if cfg.HeadroomReserve <= 0 {
		return nil, nil
	}

	sess, err := headroom.Session(repo.Path)
	if err != nil {
		return nil, &Problem{Repo: repo.Name, Err: fmt.Errorf(
			"could not open mdsmith session for headroom: %w", err)}
	}

	content := map[string][]byte{}
	for _, f := range files {
		content[f.OID] = f.Content
	}

	out := map[int64]int{}
	for _, e := range entries {
		v := e.Primary()
		if v.Plan.Done() || v.Plan.Superseded() {
			continue
		}

		src, ok := content[v.OID]
		if !ok {
			continue
		}

		reserve := headroom.ReserveLines(src, cfg.HeadroomReserve)
		room, err := headroom.Room(sess, v.Path, src, reserve)
		if err != nil || room >= reserve {
			continue
		}
		out[e.Key.ID] = reserve - room
	}

	return out, nil
}

// fetchRemote refreshes remote's tracking refs so Gather reads landed
// evidence off a fresh copy rather than a stale one. A repository with
// no such remote configured has nothing to refresh — the same "no
// remote" case laggingDefaultBranch treats as benign — so it is skipped
// rather than attempted. The error of an attempt that failed is
// returned rather than fatal: Gather falls back to the local view and
// names the staleness, so one unreachable remote does not blind the
// whole walk.
//
// The configured check is a listing, not a probe of remote by name:
// `remote get-url <remote>` cannot tell "not configured" from "the
// probe itself failed" by error alone, and folding the two together
// silently skips a fetch that staleFetch should have named instead.
func fetchRemote(dir, remote string, run gitwt.Runner) error {
	names, err := run(dir, "remote")
	if err != nil {
		return err
	}
	if !hasRemoteName(string(names), remote) {
		return nil
	}

	_, err = run(dir, "fetch", "--prune", "--quiet", remote)

	return err
}

// hasRemoteName reports whether out — `git remote`'s one-name-per-line
// listing — names remote.
func hasRemoteName(out, remote string) bool {
	return slices.Contains(
		strings.Split(strings.TrimSpace(out), "\n"), remote)
}

// staleFetch names a fetch that was attempted but failed, so a reader
// knows the remote-tracking view may be stale rather than trusting it
// silently. It is reported only when the ref list carries a
// refs/remotes/<remote>/* ref — the mark that this remote has been
// fetched before, so its view is one a fetch was meant to refresh. A
// remote configured but never fetched has no such ref, and its first
// failed fetch leaves nothing that was fresh to go stale.
func staleFetch(
	repo discover.Repo, remote string, fetchErr error, refs []gitobj.Ref,
) *Problem {
	if fetchErr == nil || !hasRemoteTracking(refs, remote) {
		return nil
	}

	return &Problem{
		Repo: repo.Name,
		Err: fmt.Errorf(
			"could not fetch %s; remote-tracking view may be stale", remote),
	}
}

// hasRemoteTracking reports whether the ref list carries any
// refs/remotes/<remote>/* ref, which a repository has exactly when that
// remote is configured and has been fetched at least once.
func hasRemoteTracking(refs []gitobj.Ref, remote string) bool {
	prefix := "refs/remotes/" + remote + "/"
	for _, r := range refs {
		if strings.HasPrefix(r.Name, prefix) {
			return true
		}
	}

	return false
}

// coordOf resolves the lease coordinate from what the gather already
// read: the repository path, the config's remote, and the base — the
// config's when set, otherwise the default-ref cascade the gather ran.
func coordOf(repo discover.Repo, cfg repocfg.Config, preferred string) Coord {
	base := cfg.Base
	if base == "" {
		base = preferred
	}

	return Coord{
		Path: repo.Path, Remote: cfg.Remote, Base: base,
		TakeoverWindow: cfg.TakeoverWindow, SampleGap: cfg.SampleGap,
	}
}

// laggingDefaultBranch reports when a repository's local default
// branch is a strict ancestor of its own already-fetched
// remote-tracking ref — `git fetch` ran, the merge did not — using
// only the ref list Gather already read (S80). It is nil whenever
// there is nothing to compare: no local copy, no remote-tracking
// copy at all (never fetched, or no remote), or the two already
// agree.
func laggingDefaultBranch(
	repo discover.Repo, remote, preferred string, refs []gitobj.Ref,
	run gitwt.Runner,
) *Problem {
	branch, ok := gitobj.Ref{Name: preferred}.Branch()
	if !ok {
		return nil
	}

	local := refOID(refs, "refs/heads/"+branch)
	trackingName := "refs/remotes/" + remote + "/" + branch
	tracking := refOID(refs, trackingName)
	if local == "" || tracking == "" || local == tracking {
		return nil
	}

	if !isAncestor(repo.Path, local, tracking, run) {
		return nil
	}

	return &Problem{
		Repo: repo.Name,
		Err: fmt.Errorf(
			"local default branch is %d commit(s) behind fetched %s; "+
				"fetch ran, merge did not",
			commitsBehind(repo.Path, local, tracking, run), trackingName),
	}
}

// refOID returns the object a named ref points at, or "" when the
// ref list carries no such ref.
func refOID(refs []gitobj.Ref, name string) string {
	for _, r := range refs {
		if r.Name == name {
			return r.OID
		}
	}

	return ""
}

// isAncestor reports whether sha is an ancestor of base. A non-zero
// exit — not an ancestor, or any git fault — reads as false, the
// safe default: a lag is only ever reported on a clear yes.
func isAncestor(dir, sha, base string, run gitwt.Runner) bool {
	_, err := run(dir, "merge-base", "--is-ancestor", sha, base)

	return err == nil
}

// commitsBehind counts the commits sha would gain by fast-forwarding
// to base. A count that cannot be read answers 0 rather than failing
// the whole gather over a report's wording.
func commitsBehind(dir, sha, base string, run gitwt.Runner) int {
	out, err := run(dir, "rev-list", "--count", sha+".."+base)
	if err != nil {
		return 0
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}

	return n
}

// heldBranches maps each claimed plan id to the branches that claim it:
// the same holds the orphan report is built on, merged refs already
// filtered out so landed work does not read as a live claim. The branch
// names are the lanes a holder works the plan on, deduplicated so a
// claim pushed to a remote does not read as two. Beside the branches it
// returns each plan's lease tip — the commit its id-only work ref
// points at — which is what the staleness observer watches and exactly
// what a takeover CASes on.
func heldBranches(
	repo discover.Repo, cfg repocfg.Config, preferred string,
	refs []gitobj.Ref, landed map[int64]bool, run gitwt.Runner,
) (map[int64][]string, map[int64]string, error) {
	holds, err := cfg.Compiled()
	if err != nil {
		return nil, nil, err
	}
	merged, err := gitobj.MergedRefs(repo.Path, preferred, run)
	if err != nil {
		return nil, nil, err
	}

	tips := map[string]string{}
	for _, r := range refs {
		tips[r.Name] = r.OID
	}

	held := map[int64][]string{}
	for _, lane := range lanes.Build(
		repo.Worktrees, refs, merged, landed, holds) {
		seen := map[string]bool{}
		for _, h := range lane.Holds {
			if seen[h.Branch] {
				continue
			}
			seen[h.Branch] = true
			// Held alone already tells a live lease apart from one that
			// ended (its nearest terminal marker is a release, so it
			// answers false) or was never minted (a name match with no
			// marker reachable at all) — one read instead of Released
			// and Held each walking the same ref's history.
			if !claim.Held(repo.Path, tips[h.Ref], lane.PlanID, run) {
				continue
			}
			held[lane.PlanID] = append(held[lane.PlanID], h.Branch)
		}
	}

	return held, leaseTips(refs, holds), nil
}

// leaseTips maps each plan to the tip of its id-only work ref — read
// off the raw ref list, not the hold filters, because the observer
// watches the ref itself and scavenge needs the tip of exactly the
// states the filters drop: released, merged, landed. When both a
// local and a remote-tracking copy exist the remote-tracking one
// wins: origin is the arbiter, and its copy is the lease observed.
func leaseTips(refs []gitobj.Ref, holds repocfg.Holds) map[int64]string {
	tips := map[int64]string{}
	for _, r := range refs {
		branch, ok := r.Branch()
		if !ok {
			continue
		}
		id, ok := holds.Match(branch)
		if !ok || branch != claim.Branch(id) || r.OID == "" {
			continue
		}
		if tips[id] == "" || strings.HasPrefix(r.Name, "refs/remotes/") {
			tips[id] = r.OID
		}
	}

	return tips
}

// planOf projects a plan's authoritative version into the discovery
// view, tagging it held when a lane claims its id.
func planOf(
	repoName string, e index.Entry, held map[int64][]string,
	leaseTips map[int64]string, headrooms map[int64]int,
) discovery.Plan {
	v := e.Primary()
	holds := held[e.Key.ID]

	return discovery.Plan{
		Key:           e.Key.String(),
		Repo:          repoName,
		ID:            e.Key.ID,
		Status:        v.Plan.Status,
		Title:         v.Plan.Title,
		Summary:       v.Plan.Summary,
		Model:         v.Plan.Model,
		Goal:          v.Plan.Goal,
		DependsOn:     v.Plan.DependsOn,
		Phases:        v.Plan.Phases,
		Path:          v.Path,
		Branches:      shortBranches(e),
		Held:          len(holds) > 0,
		Holds:         holds,
		HoldTip:       leaseTips[e.Key.ID],
		HeadroomShort: headrooms[e.Key.ID],
	}
}

// shortBranches collects the short names of every ref carrying the
// plan, in any version, deduplicated and sorted so a selector's slug
// match is stable. These are what a plan is remembered by when the id
// is gone.
func shortBranches(e index.Entry) []string {
	seen := map[string]bool{}
	for _, v := range e.Versions {
		for _, ref := range v.Refs {
			seen[gitobj.Ref{Name: ref}.Short()] = true
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)

	return out
}
