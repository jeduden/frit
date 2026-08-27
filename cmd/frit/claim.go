package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
)

type claimCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run mints frit's own hold on a plan — the atomic claim the dispatch
// ladder's rung 3 stands a lane on. It refuses a plan that is not
// startable: one already held, one blocked by an unfinished dependency,
// or one another machine wins the push for. On a pickable plan it leases
// the branch and reports the base it dated the lease against.
//
// The claim mints the ref and stops. The worktree and the pane are
// herdr's, reached by `frit start`; this command owns only the lease.
func (cc *claimCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	// The gather above reads the whole fleet, and rightly wants each
	// repository's fetch bounded independently. What follows is scoped
	// to one repository's own lease ref, so it shares a single deadline
	// instead: a stalled remote should cost roughly --git-timeout, not
	// a multiple of it across the pre-push read, the push and a retry.
	rt.git = gitwt.WithDeadline(gitwt.Exec, time.Now().Add(c.GitTimeout))

	plan, err := resolveSelector(rt, cc.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	branch := claim.Branch(plan.ID)
	doc := report.NewClaim(c.Root, plan.Repo, plan.ID, plan.Title, branch)
	carryProblems(doc, res.Problems, c.All)

	// The gather withholds a coordinate when two checkouts share the
	// plan's repository name; without one there is no repository to mint
	// the lease in, so refuse rather than guess.
	coord, ok := res.Coords[plan.Repo]

	// The lane's own lease, resumed on the token it persisted — ahead
	// of the "already held" refusal below, since the plan a resume
	// asks about is exactly the one this lane already holds (F9, F11,
	// S3, S21).
	cwd, _ := os.Getwd()
	if ok && resumeOwnLease(rt, doc, plan, coord, cwd) {
		return renderClaim(c, rt, doc)
	}

	// A deserted hold read from this exact lane is named before the
	// ordinary readiness check ever runs, the same guard start's
	// buildStart applies: S76 already makes a dead, unmatured hold
	// Ready for a takeover from elsewhere, but taking it over from its
	// own dead lane would leave whatever that lane committed locally,
	// past its persisted token, orphaned rather than parked (S77).
	if reason := desertedRefusal(rt, plan, cwd); reason != "" {
		doc.Refuse(reason)
		return renderClaim(c, rt, doc)
	}

	if reason := parkFirstRefusal(rt, plan, coord); reason != "" {
		doc.Refuse(reason)
		return renderClaim(c, rt, doc)
	}

	window, _ := staleClock(&res, plan.Repo)
	if reason := claimRefusal(plan, discovery.Ready(res.Plans), window); reason != "" {
		doc.Refuse(reason)
		scavengeGlyph(rt, doc, plan, res)
		return renderClaim(c, rt, doc)
	}

	if !ok {
		doc.Refuse(ambiguousRepo(plan.Repo))
		return renderClaim(c, rt, doc)
	}

	if err := mintClaim(rt, doc, plan, coord); err != nil {
		return err
	}
	if doc.Claimed {
		standUpClaimWorktree(rt, doc, plan, branch, coord)
	}

	return renderClaim(c, rt, doc)
}

// resumeOwnLease resumes the lease this very lane already holds, on
// the proof that survives a process: the token in its own git dir
// (F9, F11, S3, S21). Any doubt along the way answers false and falls
// through to the ordinary path, where the CAS is still the arbiter.
func resumeOwnLease(
	rt *runtime, doc *report.ClaimDoc,
	plan discovery.Plan, coord fleet.Coord, cwd string,
) bool {
	lane, tip, ok := resumeToken(rt, plan, coord, cwd)
	if !ok {
		return false
	}
	opts := claim.LeaseOptions{
		PlanID:  plan.ID,
		Remote:  coord.Remote,
		Base:    coord.Base,
		Holder:  hostname(),
		Lane:    lane,
		Session: currentSession(rt),
	}
	if _, err := claim.Resume(coord.Path, opts, tip, rt.git); err != nil {
		return false
	}
	doc.MarkResumed()

	return true
}

// resumeToken is ownToken further gated by the live-session veto: a
// resume takes over from wherever a competing takeover would be
// vetoed, so the same live-bound-session check applies before it
// hands its own lease back to a fresh process (F9, F11, S3, S21). ok
// is false when either ownToken fails or a live session is bound,
// leaving the ordinary mint path as the arbiter. Shared by claim,
// which stops here, and start, which resumes and then still stands
// the lane up.
func resumeToken(
	rt *runtime, plan discovery.Plan, coord fleet.Coord, cwd string,
) (lane, tip string, ok bool) {
	lane, tip, ok = ownToken(rt, plan, coord, cwd)
	if !ok {
		return "", "", false
	}

	opts := claim.LeaseOptions{
		PlanID:  plan.ID,
		Remote:  coord.Remote,
		Base:    coord.Base,
		Holder:  hostname(),
		Lane:    lane,
		Session: currentSession(rt),
	}
	if m, mOK := claim.ReadMarker(coord.Path, opts, tip, rt.git); mOK &&
		herdr.SessionLive(rt.herdr, m.Session) {
		return "", "", false
	}

	return lane, tip, true
}

// ownToken resolves this lane's own already-held lease from its
// persisted token: the calling directory is this exact plan's own
// lane, and its token either matches origin's current tip or the tip
// is this lane's own advance beyond it — an ordinary run of raw TDD
// commits, not a foreign move (claim.OwnAdvance). It is not
// identity-based — a cloned machine id or a reused lane path carries
// no matching token, so it gets no shortcut (A1) — and it consults no
// staleness window at all. ok is false when either condition fails.
// The token proof every verb that acts on a lane's own lease reads:
// resume, by resumeToken's added veto, and release, which reads it
// directly since there is no competing takeover for it to defer to.
func ownToken(
	rt *runtime, plan discovery.Plan, coord fleet.Coord, cwd string,
) (lane, tip string, ok bool) {
	// The guard against the CLI being invoked elsewhere: the same
	// cwd-join-backwards yield's tearDownLane uses to confirm the
	// calling directory is this exact plan's own lane before trusting
	// anything local it finds there.
	if !inOwnLane(rt, plan, cwd) {
		return "", "", false
	}

	lane = herdr.Resolve(cwd, rt.git).Root
	if lane == "" {
		return "", "", false
	}
	token := claim.ReadToken(lane, plan.ID, rt.git)
	if token == "" {
		return "", "", false
	}
	// Origin is read fresh rather than trusting plan.HoldTip, which is
	// this clone's possibly-stale local view of the ref: the protocol
	// states the rule against origin's current tip.
	tip = claim.RemoteTip(coord.Path, coord.Remote, plan.ID, rt.git)
	if tip == "" {
		return "", "", false
	}
	if tip == token {
		return lane, tip, true
	}
	if claim.OwnAdvance(coord.Path, plan.ID, token, tip, rt.git) {
		return lane, tip, true
	}

	return "", "", false
}

// inOwnLane reports whether cwd is this exact plan's own worktree —
// ownToken's identity check, without requiring its token to still
// match origin's tip. It is the seam a deserted-hold refusal needs:
// telling "not my lane" apart from "my lane, but its token can no
// longer resume it" (S77).
func inOwnLane(rt *runtime, plan discovery.Plan, cwd string) bool {
	if cwd == "" {
		return false
	}
	repo, id, idOK := fleet.CurrentPlanID(cwd, rt.git, holdsForRoot)

	return idOK && repo == plan.Repo && id == plan.ID
}

// currentSession is the herdr session the calling pane runs, "" when
// herdr is unreachable or no agent is on it. Best-effort: an unbound
// lease still holds, it only forgoes the veto until a later renewal
// binds one.
func currentSession(rt *runtime) string {
	pane, err := herdr.CurrentPane(rt.herdr)
	if err != nil {
		return ""
	}

	return pane.Session
}

// standUpClaimWorktree hands the freshly claimed lane's checkout to
// herdr, so an agent works it in a worktree of its own rather than in
// the shared clone. The lease is already atomic and minted; a herdr that
// cannot stand the worktree up is recorded as a warning, not a failure,
// so a lost checkout never reads as a lost claim. The agent and its
// prompt stay start's rung — claim stands up the checkout only.
func standUpClaimWorktree(
	rt *runtime, doc *report.ClaimDoc,
	plan discovery.Plan, branch string, coord fleet.Coord,
) {
	path := defaultLanePath(coord.Path, plan.Path)
	if _, err := herdr.WorktreeCreate(rt.herdr, herdr.WorktreeSpec{
		CWD:    coord.Path,
		Branch: branch,
		Base:   coord.Base,
		Path:   path,
		Label:  fmt.Sprintf("plan %d", plan.ID),
	}); err != nil {
		doc.Warn(fmt.Sprintf("worktree not stood up: %v", err))
		return
	}
	doc.Stood(path)
}

// mintClaim acquires the lease from the coordinate the gather already
// resolved — the repository path, its remote and the base — so the
// claim reads them off the one fleet walk rather than a second one. A
// held plan whose takeover window matured is seized through the
// takeover CAS instead of acquired. A lost race is the one expected
// non-fatal outcome — another machine got there first, or a quiet
// holder renewed — so it is carried in the document as a refusal
// rather than surfaced as a command failure; a git fault is returned.
func mintClaim(
	rt *runtime, doc *report.ClaimDoc,
	plan discovery.Plan, coord fleet.Coord,
) error {
	opts := claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: coord.Remote,
		Base:   coord.Base,
		Holder: hostname(),
		Lane:   defaultLanePath(coord.Path, plan.Path),
	}
	minted, err := mintOrTakeOver(rt, plan, coord, opts)
	if err != nil {
		if errors.Is(err, claim.ErrLostRace) {
			doc.Refuse(lostRaceRefusal(err))
			scavengeLanded(rt, doc, plan, coord, err)
			return nil
		}
		return err
	}
	doc.Minted(minted.BaseSHA)

	return nil
}

// scavengeLanded cleans the ref behind a lost race whose winner has
// already merged into the base — ancestry evidence, tied to the very
// tip the refusal read, so no window is needed and a holder that
// renewed since fails the CAS harmlessly.
func scavengeLanded(
	rt *runtime, doc scavengeReporter,
	plan discovery.Plan, coord fleet.Coord, err error,
) {
	var held *claim.HeldError
	if !errors.As(err, &held) || !held.Landed || held.Tip == "" {
		return
	}
	scavengeRef(rt, doc, plan, coord, held.Tip)
}

// scavengeGlyph cleans a lingering ref whose plan is already done on
// the default branch — the squash-merge shape ancestry cannot see.
// The hold filters already dropped such a ref, so Held is false here;
// the ref itself still stands, carried by HoldTip. Glyph evidence is
// not tied to the tip, so it additionally requires a matured window:
// a live, renewing holder is never scavenged (A2).
func scavengeGlyph(
	rt *runtime, doc scavengeReporter,
	plan discovery.Plan, res fleet.Result,
) {
	if !plan.Done() || !plan.Stale || plan.HoldTip == "" {
		return
	}
	coord, ok := res.Coords[plan.Repo]
	if !ok {
		return
	}
	scavengeRef(rt, doc, plan, coord, plan.HoldTip)
}

// scavengeReporter is what a scavenge records itself into: claim's
// refusal, or release's own report of a landed hold. Both carry the
// same two facts — a warning on a failed scavenge, or the ref and
// rescue a clean one found.
type scavengeReporter interface {
	Warn(reason string)
	ScavengedRef(branch, rescue string)
}

// scavengeRef runs the scavenge and records what it did. A failure is
// a warning, never a command failure: the caller's own report already
// stands, and the ref will be met again.
func scavengeRef(
	rt *runtime, doc scavengeReporter,
	plan discovery.Plan, coord fleet.Coord, tip string,
) {
	sc, err := claim.Scavenge(coord.Path, claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: coord.Remote,
		Base:   coord.Base,
		Holder: hostname(),
		Lane:   defaultLanePath(coord.Path, plan.Path),
	}, tip, rt.git)
	if err != nil {
		doc.Warn(fmt.Sprintf("scavenge: %v", err))
		return
	}
	doc.ScavengedRef(claim.Branch(plan.ID), sc.Rescue)
}

// mintOrTakeOver runs the transition the plan's state calls for:
// takeover when the hold is observed stale or its bound session
// herdr confirms gone (S76), acquire otherwise. A takeover CASes on
// exactly the tip the window observed; when a renewal wins that race
// the loser re-reads and resets — the fresh tip is folded into the
// observation store, so the window honestly starts over on what
// actually holds the ref.
func mintOrTakeOver(
	rt *runtime, plan discovery.Plan, coord fleet.Coord,
	opts claim.LeaseOptions,
) (claim.Lease, error) {
	if !plan.Held || (!plan.Stale && !plan.Dead) {
		return claim.Acquire(coord.Path, opts, rt.git)
	}

	// Held and (matured or confirmed dead): the one place a herdr veto
	// can change the answer. The marker is read from the exact tip the
	// window matured on and Takeover CASes against, so the veto and the
	// takeover reason about the same state; if origin moved since, the
	// takeover loses its CAS below and resetWindow handles it as
	// always.
	if m, ok := claim.ReadMarker(coord.Path, opts, plan.HoldTip, rt.git); ok &&
		herdr.SessionLive(rt.herdr, m.Session) {
		return claim.Lease{}, &claim.VetoError{
			PlanID:  plan.ID,
			Marker:  m,
			Renewed: beatForHolder(rt, coord, opts, m, plan.HoldTip),
		}
	}

	lease, err := claim.Takeover(coord.Path, opts, plan.HoldTip, rt.git)
	var held *claim.HeldError
	if errors.As(err, &held) && held.Tip != "" {
		resetWindow(plan, held.Tip, time.Now())
	}

	return lease, err
}

// beatForHolder renews a vetoed lease on its own holder's behalf: a
// beat CASed from the observed tip, same epoch. It reports whether the
// push landed, so the refusal does not claim a renewal it did not
// make.
//
// Every identity trailer is copied off the holder's marker, never
// taken from this run: the beat renews the holder's lease, not this
// machine's, and stamping this run's own holder and lane on it would
// make the ref claim a holder it does not have — the identity
// confusion A1 and F10 warn against, and the wrong answer for board
// and orphans.
func beatForHolder(
	rt *runtime, coord fleet.Coord, opts claim.LeaseOptions,
	m claim.Marker, tip string,
) bool {
	beatOpts := claim.LeaseOptions{
		PlanID:  opts.PlanID,
		Remote:  opts.Remote,
		Base:    opts.Base,
		Holder:  m.Holder,
		Lane:    m.Lane,
		Session: m.Session,
	}
	_, err := claim.Renew(coord.Path, beatOpts, tip, rt.git)

	return err == nil
}

// resetWindow restarts a plan's observation window on the tip a lost
// takeover re-read, so the store watches what actually holds the ref
// rather than re-offering a takeover the server already refused.
// Best-effort, like all observation.
func resetWindow(plan discovery.Plan, tip string, now time.Time) {
	path, err := observe.Path()
	if err != nil {
		return
	}
	state := observe.Load(path)
	key := observe.Key(plan.Repo, plan.ID)
	state[key] = discovery.Observe(
		discovery.Window{}, tip, now, discovery.DefaultSampleGap)
	_ = observe.Save(path, state)
}

// lostRaceRefusal names who holds a plan whose acquire lost, from the
// facts claim.Acquire read off the winning lease's marker. A work ref
// that already landed, a lease this machine holds, and one held
// elsewhere each read differently; an unread marker falls back to the
// original wording so a missing or malformed body never changes the
// outcome.
func lostRaceRefusal(err error) string {
	var veto *claim.VetoError
	if errors.As(err, &veto) {
		return vetoRefusal(veto)
	}

	var held *claim.HeldError
	if !errors.As(err, &held) || !held.Known {
		return "lost the race to another machine"
	}

	switch {
	case held.Landed:
		return fmt.Sprintf(
			"the claim branch has already landed; its status is still open, "+
				"so set plan %d to ✅", held.PlanID)
	case held.ThisHolder:
		return fmt.Sprintf("already held on this host (%s)", held.Marker.Holder)
	case held.Marker.Holder != "":
		return fmt.Sprintf(
			"lost the race to another machine (%s)", held.Marker.Holder)
	default:
		// The marker was read but carried no holder — a malformed body.
		// Name no machine rather than print an empty pair of parentheses.
		return "lost the race to another machine"
	}
}

// vetoRefusal names the live holder a takeover was refused for, and
// says whether its lease was renewed on its behalf — the refusal must
// not claim a renewal that did not land.
func vetoRefusal(veto *claim.VetoError) string {
	who := veto.Marker.Holder
	if who == "" || who == "-" {
		who = "another machine"
	}
	if veto.Renewed {
		return fmt.Sprintf(
			"is held by a live agent session on %s; "+
				"its lease was renewed on its behalf", who)
	}

	return fmt.Sprintf("is held by a live agent session on %s", who)
}

// claimRefusal reports why a plan cannot be claimed, or "" when it can.
// A plan is claimable when it is startable — not begun, held by nobody,
// every dependency done — so membership in the ready set is the test.
// One state outside that set is claimable anyway: a plan in progress that
// nobody holds. Its lane vanished when its first phase merged — the 🔳
// marker rode in on the merge — leaving prescribed work with no lane to
// resume it on. The Held case is checked before InProgress, so reaching
// the latter means nobody holds it; the resume re-acquires the hold on
// the deterministic branch, and Acquire's force-with-lease stays the
// arbiter if a live hold still exists. Every other plan outside the
// ready set is refused, and the reason names why it is out.
func claimRefusal(
	p discovery.Plan, ready []discovery.Plan, window time.Duration,
) string {
	for _, r := range ready {
		if r.Repo == p.Repo && r.ID == p.ID {
			return ""
		}
	}

	switch {
	case p.Held:
		return "already held (" + heldLabel(p.Holds) + "); " +
			notMaturedReason(p, window)
	case p.Done():
		return "already done"
	case p.Superseded():
		return "superseded"
	case p.InProgress():
		// In progress and, since Held was excluded above, unheld: a resume.
		return ""
	default:
		return fmt.Sprintf(
			"blocked by an unfinished dependency; see frit show %d", p.ID)
	}
}

// defaultLanePath is where the lane's worktree lives by convention: a
// sibling of the repository named for it, `<repo>-<slug>`, with the
// slug taken from the plan file name after its id prefix — the branch
// carries the id alone, so the human-readable name comes from the
// file. frit does not create it — that is herdr's worktree.create —
// but the marker records it so the lane's history names where the work
// will live, the same path the dispatch ladder hands to herdr.
func defaultLanePath(repoPath, planPath string) string {
	stem := strings.TrimSuffix(filepath.Base(planPath), ".md")
	slug := stem
	if i := strings.IndexByte(stem, '_'); i >= 0 {
		slug = stem[i+1:]
	}

	return filepath.Join(filepath.Dir(repoPath),
		filepath.Base(repoPath)+"-"+slug)
}

// renderClaim prints the claim as a table or emits it as JSON.
func renderClaim(c *cli, rt *runtime, doc *report.ClaimDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printClaim(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printClaim reports the lease that was minted, or why one was not. A
// refusal is not a failure — it is the honest answer that the plan was
// not this run's to take — so the command still exits clean.
func printClaim(out io.Writer, doc *report.ClaimDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		if doc.Scavenged != "" {
			_, _ = fmt.Fprintf(out, "  scavenged: %s\n", doc.Scavenged)
		}
		if doc.Rescue != "" {
			_, _ = fmt.Fprintf(out, "  rescued:   %s\n", doc.Rescue)
		}
		if doc.Warning != "" {
			_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
		}
		return
	}

	head := "claimed plan %d\n  branch: %s\n"
	if doc.Resumed {
		head = "resumed plan %d\n  branch: %s\n"
	}
	_, _ = fmt.Fprintf(out, head, doc.Plan.ID, doc.Branch)
	if doc.Base != "" {
		_, _ = fmt.Fprintf(out, "  base:   %s\n", doc.Base)
	}
	if doc.Worktree != "" {
		_, _ = fmt.Fprintf(out, "  worktree: %s\n", doc.Worktree)
	}
	if doc.Warning != "" {
		_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
	}
}
