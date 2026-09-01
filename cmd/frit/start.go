package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
)

// startTimeoutMS bounds how long herdr waits for a started agent to come
// up before failing. It is a wait on the agent's readiness, not on a
// reply — the escalation prompts only after the agent answers ready.
const startTimeoutMS = 120000

type startCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
	Phase    string `help:"Phase to dispatch; default is the plan's next open phase."`
	Note     string `help:"A rider folded into the composed prompt before it is sent."`
	Edit     bool   `help:"Open the composed prompt in $EDITOR before sending it."`
	Go       bool   `help:"Run the escalation; without it, start only prints what it would do."`
}

// Run composes the full escalation — rung three — and, with --go, runs
// it: mint the claim, stand the worktree and agent up through herdr,
// prompt the agent, and focus the pane. It is dry-run by default: without
// --go it prints the whole plan and spawns nothing, so the escalation
// stays auditable before anything runs. A plan that is not startable is
// refused, and the tier is read from the plan, never chosen here.
//
// Every mutation is delegated where it is not frit's: frit mints the
// claim, and hands the worktree, the agent and the pane to herdr. It
// never reads a reply.
func (s *startCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, s.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	return startResolved(c, rt, res, plan, s.Phase, s.Note, s.Edit, s.Go)
}

// startResolved composes and, under doGo, runs the escalation for a plan
// already chosen — whether start resolved it from a selector or pick
// ranked it to the top — and renders the result. A lost race is rendered
// as the refusal it is; only pick --go retries past it.
func startResolved(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phaseSel, note string, edit, doGo bool,
) error {
	doc, _, err := buildStart(
		c, rt, res, plan, phaseSel, note, edit, doGo, true)
	if err != nil {
		return err
	}

	return renderStart(c, rt, doc)
}

// buildStart composes the escalation doc for a plan already chosen and,
// under doGo, runs start's claim-and-stand-up path. It refuses an
// unstartable plan and an ambiguous repository the same way for both
// verbs, so they cannot drift on what "startable" or "started" means.
// The bool is true when execution lost the claim's race — the one
// refusal pick --go retries past rather than reports.
//
// reattach is whether a held lane may be resumed from outside it, off
// its hold's own marker (#122): true for an explicit `start <id>`,
// where the caller named the lane; false for pick --go, which ranks
// ready plans and promises to resume only an unheld one — a held
// checkout is never quietly reopened on the way to the top candidate.
func buildStart(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phaseSel, note string, edit, doGo, reattach bool,
) (*report.StartDoc, bool, error) {
	phase, ok := dispatch.Phase(plan.Phases, phaseSel)
	if !ok {
		return nil, false, fmt.Errorf(
			"plan %d has no open phase; pass --phase", plan.ID)
	}

	// The gather withholds a coordinate when two checkouts share the
	// plan's repository name; without one there is no repository to stand
	// the lane in, so a resume cannot be checked either.
	coord, coordOK := res.Coords[plan.Repo]
	cwd, _ := os.Getwd()
	rs := startResume(rt, plan, coord, coordOK, cwd, reattach)

	// Refuse before reading the repository off disk: a plan already held
	// or blocked needs no base, worktree path or git subprocess.
	if doc := startRefusal(c, rt, res, plan, phase, doGo, coord, cwd, rs); doc != nil {
		return doc, false, nil
	}

	if !coordOK {
		return refusedStart(
			c, res, plan, phase, doGo, ambiguousRepo(plan.Repo)), false, nil
	}

	liveDoc, liveProbs, liveHerdrErr := startLiveLaneRefusal(
		c, rt, res, plan, phase, doGo, rs)
	if liveDoc != nil {
		return liveDoc, false, nil
	}

	sc := startContextOf(coord)
	sp := composeStart(plan, phase, note, sc)
	if rs.active() {
		// The report names the lane the resume renews on and reopens —
		// the checkout's real location, off the hold's marker or the
		// caller's cwd — never the naming convention composeStart
		// computes, which may point at a directory nothing runs in.
		sp.Lane = rs.Lane
	}
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title, sp, doGo)
	carryProblems(doc, res.Problems, c.All)
	carryLiveLaneProblems(doc, liveProbs, liveHerdrErr)
	if rs.active() {
		doc.MarkResumed()
	}

	if doGo {
		if err := startExecute(
			rt, doc, plan, sp, sc, edit, rs,
		); err != nil {
			if lostRace(err) {
				doc.Refuse(lostRaceRefusal(err))
				scavengeLanded(rt, doc, plan, coord, err)

				return doc, true, nil
			}

			return nil, false, err
		}
	}

	return doc, false, nil
}

// startRefusal is the refusal buildStart renders before it reads the
// repository off disk, nil when the plan is startable. A resumable own
// lease skips the readiness refusals — it is startable by definition,
// whether or not its window has matured — but a reattach still meets
// the park-first guard: its renewal moves the shared work ref exactly
// as a takeover would, so a suffix the dead lane never pushed would be
// orphaned the same way (S77).
func startRefusal(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phase string, doGo bool, coord fleet.Coord, cwd string, rs startResumption,
) *report.StartDoc {
	if rs.Reattach {
		if reason := reattachParkFirstRefusal(rt, plan, coord, rs); reason != "" {
			return refusedStart(c, res, plan, phase, doGo, reason)
		}

		return nil
	}
	if rs.active() {
		return nil
	}
	// A deserted hold read from this exact lane is named before the
	// ordinary readiness check ever runs: S76 already makes a dead,
	// unmatured hold Ready for a takeover from elsewhere, but taking
	// it over from its own dead lane would leave whatever that lane
	// committed locally, past its persisted token, orphaned rather
	// than parked — yield is the way out, not a silent takeover
	// (S77).
	if reason := desertedRefusal(rt, plan, cwd); reason != "" {
		return refusedStart(c, res, plan, phase, doGo, reason)
	}
	if reason := parkFirstRefusal(rt, plan, coord); reason != "" {
		return refusedStart(c, res, plan, phase, doGo, reason)
	}
	window, _ := staleClock(&res, plan.Repo)
	if reason := claimRefusal(plan, discovery.Ready(res.Plans), window); reason != "" {
		doc := refusedStart(c, res, plan, phase, doGo, reason)
		scavengeGlyph(rt, doc, plan, res)

		return doc
	}

	return nil
}

// startResume resolves the resume start is entitled to, the zero value
// when it is entitled to none. Two proofs answer the same question — is
// this lane already ours to pick back up — and the cheaper one goes
// first: the persisted token, read straight off the lane's own git dir
// when start runs from inside it — ahead of the "already held"
// refusal, exactly as claim orders it (F9, F11, S3, S21) — then, when
// the caller allows a reattach, the hold's own marker, which is what a
// closed pane leaves you outside the lane to read (#122). Both fail to
// the zero value when the plan carries no matching coordinate, or none
// of the resume conditions hold; start's ordinary claim path is then
// the arbiter. Lane is the checkout's real location — cwd-derived for
// the token proof, the marker's own trailer for the reattach — never
// the naming convention composeStart computes, since a resumed renewal
// must record where the checkout genuinely is: orphans and reap trust
// that record to tell a foreign checkout apart from the real one.
func startResume(
	rt *runtime, plan discovery.Plan, coord fleet.Coord, coordOK bool,
	cwd string, reattach bool,
) startResumption {
	if !coordOK {
		return startResumption{}
	}
	if lane, tip, ok := resumeToken(rt, plan, coord, cwd); ok {
		return startResumption{Lane: lane, Tip: tip}
	}
	if !reattach {
		return startResumption{}
	}
	lane, tip := laneTokenResumeTip(rt, plan, coord)

	return startResumption{Lane: lane, Tip: tip, Reattach: tip != ""}
}

// startResumption is the resume start resolved, and which proof
// resolved it. Reattach is the whole of the difference the stand-up
// cares about: the token proof fires only from inside the lane, so the
// pane to drive is the one you are standing in, while the hold's marker
// is read from outside it, where the current pane is the caller's and
// the lane's own checkout has to be put back on screen (#122).
type startResumption struct {
	Lane     string
	Tip      string
	Reattach bool
}

// active reports whether start is entitled to a resume at all — the
// question the "already held" refusal and the resume transition both
// turn on.
func (r startResumption) active() bool { return r.Tip != "" }

// laneTokenResumeTip resolves the resume from outside the lane, for a
// lane whose pane was closed (#122). The token proof resumeToken
// passes is cwd-derived, so it fires only from inside the lane; out here the
// hold's own marker says where the lane is, and the token that lane
// persisted is the proof it is this machine's lease — the same proof
// ownToken passes, found by a different route. The marker's holder
// and lane trailers are reporting, never identity: a cloned machine
// or a reused path shares the strings with no race needed, so nothing
// here gates on them (A1). The lane trailer only says where to look;
// a checkout with no token gets no shortcut and waits the window like
// any other claimant. Both return values are "" outside that exact
// case, leaving start's ordinary claim path the arbiter.
//
// A hold recording no lane is not resumed at all: there is no checkout
// to read a token from, and renewing on "-" would stamp it into the
// very trailer orphans and reap read as a path.
func laneTokenResumeTip(
	rt *runtime, plan discovery.Plan, coord fleet.Coord,
) (lane, tip string) {
	if !plan.Held {
		return "", ""
	}
	// Origin is read fresh rather than trusting plan.HoldTip, exactly as
	// ownToken does: the CAS states its rule against origin's current
	// tip, not this clone's possibly-stale view of the ref.
	tip = claim.RemoteTip(coord.Path, coord.Remote, plan.ID, rt.git)
	if tip == "" {
		return "", ""
	}
	opts := claim.LeaseOptions{
		PlanID: plan.ID, Remote: coord.Remote, Base: coord.Base}
	m, ok := claim.ReadMarker(coord.Path, opts, tip, rt.git)
	if !ok || !m.HasLane() {
		return "", ""
	}
	token := claim.ReadToken(m.Lane, plan.ID, rt.git)
	if !tokenProves(rt, coord, plan.ID, token, tip) {
		return "", ""
	}
	if !laneUnattended(rt, m) {
		return "", ""
	}

	return m.Lane, tip
}

// laneUnattended reports whether herdr positively shows no agent on a
// hold's lane, read from outside it: neither a live agent on the
// session the marker binds, nor any local agent pane sitting in the
// recorded checkout — a lane stood up by hand is occupied whether or
// not the lease ever named its session. One pane list answers both.
// Only a herdr that answered counts: from outside the lane nothing
// else vouches for it, so an unreachable herdr reads as unknown and
// keeps the window rather than resume over an agent that may still be
// working (F3, S61).
func laneUnattended(rt *runtime, m claim.Marker) bool {
	panes, err := herdr.List(rt.herdr)
	if err != nil {
		return false
	}

	return !herdr.SessionLiveIn(panes, m.Session) &&
		!herdr.LiveRoots(panes, rt.git)[m.Lane]
}

// desertedRefusal names the yield that retires a deserted hold read
// from its own lane: the plan is held, herdr confirms the bound
// session gone, the takeover window has not matured, and this exact
// worktree is the one that held it — a deserted hold, and the one
// place a bare takeover would silently orphan whatever it committed
// past its persisted token (S77). Held is checked explicitly here
// rather than assumed from Dead: observeHolds happens to set Dead
// only for a held plan today, but this refusal reads its own inputs
// rather than lean on that as an unstated invariant. "" outside that
// exact case, leaving the ordinary readiness refusal as the arbiter —
// a matured window is staleHeld's own cell, orphans.go, and a
// takeover from elsewhere is unaffected.
func desertedRefusal(rt *runtime, plan discovery.Plan, cwd string) string {
	if !plan.Held || !plan.Dead || plan.Stale {
		return ""
	}
	if !inOwnLane(rt, plan, cwd) {
		return ""
	}

	return fmt.Sprintf(
		"deserted hold: its token cannot self-resume; "+
			"run `frit yield %d` to retire this lane", plan.ID)
}

// parkFirstRefusal is the S77 park-first guard's cwd-free sibling:
// desertedRefusal already covers the in-lane case above and returns
// before this ever runs there, so this is reached only from the
// primary or any other clone that never stood in the lane. A
// herdr-confirmed-dead, unmatured hold is Ready for a takeover (S76),
// but the takeover CAS knows nothing about the branch's own local
// suffix past the token it observed — a divergent tip taken over here
// would silently orphan whatever the dead lane committed, instead of
// parking it first. "" when the plan is not that exact case, or the
// gather withheld a coordinate to read the branch from (the
// ambiguous-repo refusal is the arbiter there), leaving the ordinary
// readiness path unaffected.
func parkFirstRefusal(rt *runtime, plan discovery.Plan, coord fleet.Coord) string {
	if !plan.Held || !plan.Dead || plan.Stale || coord.Path == "" {
		return ""
	}
	if !unparkedSuffix(rt, coord.Path, plan.ID, plan.HoldTip) {
		return ""
	}

	return fmt.Sprintf(
		"deserted hold: its branch carries an unparked suffix; "+
			"run `frit yield %d` to park it first", plan.ID)
}

// reattachParkFirstRefusal is the park-first guard for a resume from
// outside the lane (S77). The lane is this host's own — its token just
// proved as much — but the resume's beat is CASed from origin's tip
// and moves the shared work ref onto it, so any commits the dead lane
// made past that tip and never pushed would be left dangling, exactly
// as a takeover would leave them. Whether the hold reads dead or not
// makes no difference here: the #122 hold that never named a session
// cannot be confirmed dead, and its suffix is orphaned just the same.
// "" when the branch carries no such suffix, or the gather withheld a
// coordinate to read it from.
func reattachParkFirstRefusal(
	rt *runtime, plan discovery.Plan, coord fleet.Coord, rs startResumption,
) string {
	if coord.Path == "" || !unparkedSuffix(rt, coord.Path, plan.ID, rs.Tip) {
		return ""
	}

	return fmt.Sprintf(
		"held lane %s carries an unparked suffix; "+
			"run `frit yield %d` from that lane to park it first",
		rs.Lane, plan.ID)
}

// unparkedSuffix reports whether the local work ref in dir has moved
// past tip — commits the lane made and never pushed, which a CAS from
// tip would silently orphan. A ref that cannot be read carries no
// suffix to protect.
func unparkedSuffix(rt *runtime, dir string, planID int64, tip string) bool {
	local, err := localRef(rt, dir, claim.Branch(planID))
	if err != nil || local == "" {
		return false
	}

	return !isAncestor(rt, dir, local, tip)
}

// isAncestor reports whether sha is reachable from base — plumbing
// only, the exit code is the answer — so a divergent local suffix can
// be told apart from ordinary history. Any git fault reads as false,
// the safe default: an unreadable relationship refuses rather than
// risks a takeover over unparked work.
func isAncestor(rt *runtime, dir, sha, base string) bool {
	_, err := rt.git(dir, "merge-base", "--is-ancestor", sha, base)

	return err == nil
}

// startLiveLaneRefusal is buildStart's live-lane pre-flight, on the
// fresh-acquire branch only: a resume (rs.active(), resumeTip != "")
// skips it outright, since the live agent it would find is the lane
// resuming its own token. It refuses when herdr already shows a live
// agent on the plan's own hold branch, and otherwise returns a nil doc
// carrying the presence read's own problems, so they can still ride
// into the eventual success doc rather than being swallowed here.
func startLiveLaneRefusal(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phase string, doGo bool, rs startResumption,
) (*report.StartDoc, []hostProblem, error) {
	if rs.active() {
		return nil, nil, nil
	}
	lane, found, hostProbs, herdrErr := liveLaneFor(c, plan, rt)
	if !found {
		return nil, hostProbs, herdrErr
	}
	doc := refusedStart(c, res, plan, phase, doGo, liveLaneRefusal(lane))
	carryLiveLaneProblems(doc, hostProbs, herdrErr)

	return doc, nil, nil
}

// carryLiveLaneProblems adds the live-lane presence read's own
// problems onto doc, whichever shape it ends up: a refusal when a live
// agent was found, or the eventual success doc when it was not, so an
// unreachable herdr or an unread host is never silently dropped either
// way.
func carryLiveLaneProblems(
	doc *report.StartDoc, probs []hostProblem, herdrErr error,
) {
	for _, p := range probs {
		doc.AddProblem(p.name, p.err)
	}
	if herdrErr != nil {
		doc.AddProblem("herdr", herdrErr)
	}
}

// liveLaneRefusal names the refusal a fresh acquire meets when herdr
// already shows a live agent on the plan's own hold branch: the
// live-but-unbound lane a session-less lease leaves the takeover veto
// unable to see, and reconcileLeftoverWorktree misses when no worktree
// is registered on the branch in this repository at all (issue #126).
func liveLaneRefusal(lane herdr.Lane) string {
	return fmt.Sprintf(
		"a live herdr pane (%s) already sits on lane %s; "+
			"free it before starting this plan again",
		lane.Pane.PaneID, lane.Branch)
}

// refusedStart composes the escalation doc for a plan buildStart is
// refusing before it reaches the repository — an unstartable plan, or
// one whose repository name is ambiguous across checkouts.
func refusedStart(
	c *cli, res fleet.Result, plan discovery.Plan,
	phase string, doGo bool, reason string,
) *report.StartDoc {
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title,
		report.StartPlan{Phase: phase, Tier: plan.Model, Kind: "claude"}, doGo)
	carryProblems(doc, res.Problems, c.All)
	doc.Refuse(reason)

	return doc
}

// startContext is the repository state the escalation reads once: where
// the repository lives, the remote a claim is pushed to, and the base a
// lease is dated against.
type startContext struct {
	repoPath string
	remote   string
	base     string
}

// coord is startContext's own fields back as the fleet.Coord releaseLease
// and startAcquire's takeover path both take — the reverse of
// startContextOf, so the two never drift into declaring the same three
// fields by hand at each call site.
func (sc startContext) coord() fleet.Coord {
	return fleet.Coord{Path: sc.repoPath, Remote: sc.remote, Base: sc.base}
}

// startContextOf reads the escalation's inputs off the coordinate the
// gather already resolved — the repository path, its remote and the
// base — so start dates a lease from the one fleet walk rather than a
// second one. It reads the same coordinate claim mints from, so the two
// never disagree on where a lease is dated from.
func startContextOf(coord fleet.Coord) startContext {
	return startContext{
		repoPath: coord.Path,
		remote:   coord.Remote,
		base:     coord.Base,
	}
}

// composeStart builds the escalation from the plan: the claim branch and
// its base, the worktree path herdr would use, the agent kind and tier,
// and the typed prompt with any note folded in. The composition is the
// same whether or not --go follows.
func composeStart(
	plan discovery.Plan, phase, note string, sc startContext,
) report.StartPlan {
	return report.StartPlan{
		Phase:  phase,
		Tier:   plan.Model,
		Kind:   "claude",
		Branch: claim.Branch(plan.ID),
		Base:   sc.base,
		Lane:   defaultLanePath(sc.repoPath, plan.Path),
		Prompt: withNote(dispatch.Command(plan.ID, phase), note),
	}
}

// startExecute runs the escalation the composition describes: mint the
// claim, then hand the checkout, the agent and the pane to herdr in turn,
// and prompt it. Each mutation frit does not own is delegated — the
// worktree and the pane are herdr's — and no reply is ever read. A claim
// lost to another machine is carried as a refusal, not a fault.
func startExecute(
	rt *runtime, doc *report.StartDoc, plan discovery.Plan,
	sp report.StartPlan, sc startContext, edit bool, rs startResumption,
) error {
	// Amend the prompt before minting anything: an editor that fails to
	// launch, or a prompt left empty, must abort with no claim pushed and
	// no lane half-built. git aborts an empty commit message the same way.
	text := sp.Prompt
	if edit {
		edited, err := openEditor(text)
		if err != nil {
			return err
		}
		text = edited
		if strings.TrimSpace(text) == "" {
			return errors.New("the edited prompt is empty; nothing was started")
		}
		doc.Prompt = text
	}

	// A resumed renewal is CASed against the lane it is actually
	// running from (rs.Lane), never the naming convention sp.Lane
	// computes: the two diverge the moment the checkout was set up off
	// that convention, or the plan file's slug has since changed, and
	// orphans/reap now trust the marker's lane: trailer to tell a
	// foreign checkout apart from the real one.
	lane := sp.Lane
	if rs.active() {
		lane = rs.Lane
	} else if err := reconcileLeftoverWorktree(rt, sc, sp, plan.ID); err != nil {
		return err
	}
	lease, err := startAcquire(rt, plan, sc, sp, lane, rs)
	if err != nil {
		// A lost race, or a veto, is returned, not swallowed: buildStart
		// records it as a refusal for start, and pick --go retries past it
		// to the next candidate. Every other error is a real fault.
		return err
	}

	pane, session, err := standUpLane(rt, plan, sp, sc.repoPath, text, rs)
	if err != nil {
		if rs.active() {
			// startAcquire's own renewal above already stands — this
			// lane already holds the lease, resumed on its own token,
			// not seized. A pane it could not find is a stand-up
			// failure, not a reason to give the lease up, so there is
			// nothing here to release, and the lane's own checkout is
			// never torn down; the unwind below is for a fresh acquire
			// or takeover's worktree.create branch only. A reattach did
			// open a pane of its own, though, so that is named rather
			// than left for the operator to find.
			if rs.Reattach {
				return reattachError(rs.Lane, pane, err)
			}

			return err
		}
		// The lease is minted but nothing answers behind it. Tear down
		// whatever herdr already stood up — the worktree and pane, if
		// any — so the abort is atomic rather than leaving a freed claim
		// over a live worktree. Only a teardown that itself fails falls
		// back to naming what was left behind; a clean unwind reports the
		// plain cause. Then release the lease — a pushed marker, never a
		// delete, so the next acquire reads epoch E+1. If the release
		// itself fails the lease is still on the remote, so that is
		// reported alongside whatever the teardown already named, rather
		// than swallowed into a silent orphan.
		if pane != "" {
			if tdErr := teardownHandoff(rt, pane); tdErr != nil {
				err = errors.Join(handoffError(sp.Lane, pane, err), tdErr)
			}
		}
		if relErr := releaseLease(
			rt, sc.coord(), plan, sp.Branch, sp.Base, sp.Lane, lease.Tip,
		); relErr != nil {
			return errors.Join(err, relErr)
		}

		return err
	}
	bindSession(rt, doc, plan, sp, sc, lane, lease.Tip, session)
	doc.MarkStarted(pane)

	return nil
}

// startAcquire runs the transition start's escalation calls for: the
// self-resume on a matching persisted token when one was found, or
// otherwise the same mint-or-takeover claim already runs — a live
// bound session vetoes a stale hold, a matured one is seized, and a
// fresh plan is acquired outright. start meets the same lease protocol
// claim does; it just goes on to stand the lane up afterward.
//
// lane is the path CASed onto the marker's own lane: trailer — the
// resumed lane's real location for a resume, sp.Lane's naming
// convention otherwise, since a fresh acquire or takeover is what
// stands the checkout up there in the first place.
func startAcquire(
	rt *runtime, plan discovery.Plan, sc startContext,
	sp report.StartPlan, lane string, rs startResumption,
) (claim.Lease, error) {
	opts := claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: sc.remote,
		Base:   sp.Base,
		Holder: hostname(),
		Lane:   lane,
	}
	if rs.active() {
		// Only a self-resume can name a session here: the calling pane
		// is the lane's own, so its session is the one the lease should
		// carry. A reattach is called from outside the lane, where that
		// same read answers with the caller's pane instead — a session
		// on someone else's terminal, which a later takeover would ask
		// herdr about and be told is alive. It renews unbound and lets
		// bindSession record the agent it is about to stand up.
		if !rs.Reattach {
			opts.Session = currentSession(rt)
		}

		return claim.Resume(sc.repoPath, opts, rs.Tip, rt.git)
	}

	return mintOrTakeOver(rt, plan, sc.coord(), opts)
}

// bindSession records the started agent's herdr session on the lease:
// a beat CASed from the tip the acquire just minted, carrying the
// session trailer, so a later takeover can ask herdr whether this
// lease's holder is still alive (F3, S61). lane is the same path
// startAcquire just CASed in, so the bind's own beat does not clobber
// a resume's accurate lane back to sp.Lane's naming convention.
//
// The renewal is RenewToBind, not Renew, because tip is routinely
// stale by the time this runs: the work ref is also the branch the
// lane's worktree checks out, and the bind cannot happen before the
// agent exists to name a session, so the agent has already begun
// committing on it. RenewToBind reconciles a CAS lost to our own hold
// and fences only on a genuinely foreign move.
//
// A failed bind is a warning, never an abort: the lane is up and
// working, the lease is valid on the remote, and an unbound lease only
// forgoes the veto and falls back to the staleness window. Tearing a
// healthy lane down over a decoration would be the worse failure.
func bindSession(
	rt *runtime, doc *report.StartDoc, plan discovery.Plan,
	sp report.StartPlan, sc startContext, lane, tip, session string,
) {
	if session == "" {
		return
	}
	if _, err := claim.RenewToBind(sc.repoPath, claim.LeaseOptions{
		PlanID:  plan.ID,
		Remote:  sc.remote,
		Base:    sp.Base,
		Holder:  hostname(),
		Lane:    lane,
		Session: session,
	}, tip, rt.git); err != nil {
		doc.AddProblem(plan.Repo, fmt.Errorf(
			"bind session %s to %s: %w", session, sp.Branch, err))
	}
}

// teardownHandoff removes the worktree and pane a failed handoff stood
// up, torn down by the workspace the pane belongs to — herdr names a
// pane <workspace>:<pane>, so the workspace is the segment before the
// colon, the one handle herdr.WorktreeRemove takes.
func teardownHandoff(rt *runtime, pane string) error {
	workspace, _, _ := strings.Cut(pane, ":")

	return herdr.WorktreeRemove(rt.herdr, workspace)
}

// handoffError names what a failed handoff left behind: the worktree
// and pane that stood up before the failure, so they can be found and
// torn down rather than guessed at. A handoff that died before herdr
// opened a pane left nothing standing and reports the cause alone.
func handoffError(lane, pane string, err error) error {
	if pane == "" {
		return err
	}

	return fmt.Errorf(
		"worktree %s and pane %s were stood up and are left behind: %w",
		lane, pane, err)
}

// reattachError names the pane a failed reattach left behind: herdr
// reopened the lane's checkout into it, and the agent then failed to
// come up. Nothing was stood up — the worktree was already there and
// stays — so only the pane is named. A reattach that died before any
// pane came back reports the cause alone.
func reattachError(lane, pane string, err error) error {
	if pane == "" {
		return err
	}

	return fmt.Errorf(
		"pane %s was opened on %s and is left behind: %w", pane, lane, err)
}

// lostRace reports whether an escalation's error is a race start
// lost rather than a fault: the lost-race family a fresh claim or
// takeover returns, or the fence a resume's renewal returns when a
// concurrent takeover moved the ref under it. Either is a refusal to
// render, and for pick --go a candidate to skip, never a fault that
// aborts the walk.
func lostRace(err error) bool {
	var fence *claim.FenceError

	return errors.Is(err, claim.ErrLostRace) || errors.As(err, &fence)
}

// releaseLease unwinds a lease minted before a handoff, or a claim's
// own worktree stand-up, that then failed: the release transition, a
// pushed marker rather than a delete, so the plan frees while the
// history stays for the next acquire to CAS on. It returns nil when
// the release lands, and an error naming the still-held ref when it
// did not take — a failed unwind is surfaced and can be found, not
// left as a silent orphan for the next run to trip over.
//
// It takes fleet.Coord rather than start's own startContext so both
// rungs — start's handoff and claim's worktree stand-up — reach one
// helper instead of each unwinding the lease its own way.
func releaseLease(
	rt *runtime, coord fleet.Coord, plan discovery.Plan,
	branch, base, lane, tip string,
) error {
	if _, err := claim.Release(coord.Path, claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: coord.Remote,
		Base:   base,
		Holder: hostname(),
		Lane:   lane,
	}, tip, rt.git); err != nil {
		return fmt.Errorf(
			"lease %s could not be released and is left on the remote; "+
				"run frit orphans to find it: %w", branch, err)
	}

	return nil
}

// agentStartAttempts bounds how many times startAgent retries a
// pane-not-ready agent start before giving up: the pane herdr just
// created losing the race with its own shell settling is transient,
// not a real fault, and rarely takes more than one retry to clear.
const agentStartAttempts = 3

// agentStartPause is startAgent's seam between retry attempts: a
// package variable set to a no-op in tests, mirroring openEditor.
var agentStartPause = func() { time.Sleep(500 * time.Millisecond) }

// startAgent starts the agent herdr's worktree.create just opened a
// pane for, retrying past herdr's own pane-not-ready transient — the
// freshly opened pane's shell not yet settled — up to agentStartAttempts
// times before giving up. Any other error is not retried: it is a real
// fault and falls straight into the caller's teardown.
func startAgent(
	rt *runtime, plan discovery.Plan, sp report.StartPlan, pane string,
) error {
	spec := herdr.AgentSpec{
		Name:      fmt.Sprintf("plan-%d", plan.ID),
		Kind:      sp.Kind,
		Pane:      pane,
		Model:     sp.Tier,
		TimeoutMS: startTimeoutMS,
	}

	var err error
	for attempt := 1; attempt <= agentStartAttempts; attempt++ {
		err = herdr.AgentStart(rt.herdr, spec)
		if err == nil || !paneNotReady(err) {
			return err
		}
		if attempt < agentStartAttempts {
			agentStartPause()
		}
	}

	return err
}

// paneNotReady reports whether err carries herdr's pane-not-ready
// signal — code agent_pane_busy, message "not an available shell" —
// the transient a freshly opened pane's shell not yet settling raises
// when the agent start immediately following worktree.create loses
// the race. herdr.AgentStart surfaces the runner's error body
// unwrapped, so it is matched by substring rather than parsed.
func paneNotReady(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	return strings.Contains(msg, "agent_pane_busy") &&
		strings.Contains(msg, "not an available shell")
}

// reconcileLeftoverWorktree finds a worktree already sitting on the
// plan's own branch — a leftover Release left behind, since it
// deletes nothing — and clears it ahead of a fresh acquire so herdr's
// own worktree.create never collides with it. A live herdr pane still
// on that worktree is left standing and refused rather than reaped
// out from under whoever is there. A dead leftover is parked, the
// same way reap.go's own reapStranded parks a stranded lane's branch
// before it deletes it, and its worktree registration is removed —
// never its branch, which by now is nothing this reconcile minted.
// nil when no worktree sits on the branch at all: the ordinary path,
// unaffected. An unreadable worktree list is a genuine fault, returned
// rather than swallowed as "no leftover": reading it as clear here
// would let startAcquire mint a claim ahead of a worktree.create that
// may still collide with a leftover this read simply failed to see.
func reconcileLeftoverWorktree(
	rt *runtime, sc startContext, sp report.StartPlan, planID int64,
) error {
	worktrees, err := gitwt.List(sc.repoPath, rt.git)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	var leftover *gitwt.Worktree
	for i := range worktrees {
		if worktrees[i].Branch == sp.Branch {
			leftover = &worktrees[i]
			break
		}
	}
	if leftover == nil {
		return nil
	}

	pane, ok, err := livePaneOn(rt, leftover.Path)
	if err != nil {
		return fmt.Errorf(
			"plan %d: could not confirm no live herdr pane sits on %s: %w",
			planID, leftover.Path, err)
	}
	if ok {
		return fmt.Errorf(
			"plan %d: a live herdr pane (%s) already sits on %s; "+
				"free it before restarting this plan",
			planID, pane, leftover.Path)
	}

	opts := claim.LeaseOptions{
		PlanID: planID, Remote: sc.remote, Base: sc.base, Holder: hostname(),
	}
	if _, err := parkBranch(
		rt, discover.Repo{Path: sc.repoPath}, opts, sp.Branch, true,
	); err != nil {
		return fmt.Errorf("park: %w", err)
	}
	if _, err := rt.git(sc.repoPath, "worktree", "remove", leftover.Path); err != nil {
		return fmt.Errorf("remove leftover worktree at %s: %w", leftover.Path, err)
	}

	return nil
}

// livePaneOn reports whether a herdr pane is currently sitting in the
// worktree rooted at root — a local pane only, the same guard
// herdr.LiveRoots uses, since a remote pane's cwd is a path on another
// host that could collide with a local one by coincidence. An
// unreadable herdr answers with an error rather than "no pane": the
// caller is about to park and delete a worktree on this verdict, and
// reading a socket failure as "clear" would risk exactly the live lane
// this check exists to protect.
func livePaneOn(rt *runtime, root string) (string, bool, error) {
	panes, err := herdr.List(rt.herdr)
	if err != nil {
		return "", false, err
	}
	for _, p := range panes {
		if p.Host != "" {
			continue
		}
		if site := herdr.Resolve(p.CWD, rt.git); site.Root == root {
			return p.PaneID, true, nil
		}
	}

	return "", false, nil
}

// laneStandUpPane is the pane standUpLane drives from, and there are
// three ways to reach one. A reattach puts the lane's own checkout back
// on screen with worktree.open and takes the pane that comes back: it
// is resolved from the hold's marker, so it runs from outside the lane,
// where the current pane is the caller's and would start the agent in
// the wrong directory (#122). A self-resume reads that current pane —
// out of the lane's own worktree it is the right one, read the way
// currentSession already does. A fresh acquire or takeover takes the
// pane herdr's worktree.create just opened. Neither resume goes near
// worktree.create: the lane's checkout already occupies that path, and
// calling it anyway would fail with "already used by worktree at
// <path>".
//
// The path reopened is the lane the resume actually renewed on, off the
// hold's own marker, never sp.Lane's naming convention — the two
// diverge the moment the checkout was set up off that convention, and
// reopening the convention's path would stand an agent up beside the
// commits rather than on them.
func laneStandUpPane(
	rt *runtime, plan discovery.Plan, sp report.StartPlan,
	repoPath string, rs startResumption,
) (string, error) {
	if rs.Reattach {
		pane, err := herdr.WorktreeOpen(rt.herdr, herdr.WorktreeSpec{
			CWD:   repoPath,
			Path:  rs.Lane,
			Label: laneLabel(plan),
		})
		if err != nil {
			return "", fmt.Errorf("worktree open: %w", err)
		}

		return pane, nil
	}
	if rs.active() {
		pane, err := herdr.CurrentPane(rt.herdr)
		if err != nil {
			return "", fmt.Errorf("current pane: %w", err)
		}

		return pane.PaneID, nil
	}

	pane, err := herdr.WorktreeCreate(rt.herdr, herdr.WorktreeSpec{
		CWD:    repoPath,
		Branch: sp.Branch,
		Base:   sp.Base,
		Path:   sp.Lane,
		Label:  laneLabel(plan),
	})
	if err != nil {
		return "", fmt.Errorf("worktree create: %w", err)
	}

	return pane, nil
}

// laneLabel names the workspace a lane comes up in, for the human
// scanning a screen full of them: the plan's repository and id, the
// same label whether the checkout is being created or reopened.
func laneLabel(plan discovery.Plan) string {
	return fmt.Sprintf("%s plan %d", plan.Repo, plan.ID)
}

// standUpLane hands the checkout, the agent, the prompt and the focus to
// herdr in turn and returns the pane it opened, and the herdr session
// the started agent was given — on failure too, once a pane exists, so
// the unwind can name what stood up. Every call here is herdr's — frit
// spawns nothing it does not hand straight over — and `agent read` is
// deliberately never among them.
//
// rs is the resume that got here, if any: standUpLane drives the pane
// it names rather than creating a worktree at a path the lane's own
// checkout already occupies.
func standUpLane(
	rt *runtime, plan discovery.Plan, sp report.StartPlan,
	repoPath, text string, rs startResumption,
) (string, string, error) {
	pane, err := laneStandUpPane(rt, plan, sp, repoPath, rs)
	if err != nil {
		return "", "", err
	}

	if err := startAgent(rt, plan, sp, pane); err != nil {
		return pane, "", fmt.Errorf("agent start: %w", err)
	}
	// The first moment a session exists: herdr assigns one when the
	// agent starts, and neither this call nor worktree.create answers
	// with it, so it is read back off the same agent list every other
	// read uses. Best-effort — a lookup that fails costs the lease
	// only its herdr veto, not the lane.
	session := herdr.PaneSession(rt.herdr, pane)

	if err := herdr.Prompt(rt.herdr, pane, text); err != nil {
		return pane, session, fmt.Errorf("prompt: %w", err)
	}
	if err := herdr.Focus(rt.herdr, pane); err != nil {
		return pane, session, fmt.Errorf("focus: %w", err)
	}

	return pane, session, nil
}

// openEditor is the seam for --edit: it hands the composed prompt to
// $EDITOR and reads back what the human left. It is a package variable so
// a test can amend the prompt without a real editor on the machine.
var openEditor = editInEditor

// editInEditor writes the prompt to a temp file, opens it in $EDITOR, and
// returns the edited contents. It is the git-commit-message pattern: a
// prefilled template to amend, not an empty box.
func editInEditor(prompt string) (string, error) {
	// $VISUAL wins over $EDITOR, the shell convention, and both may carry
	// flags — "code --wait", "emacsclient -c" — so the value is split into
	// a command and its arguments rather than treated as one binary name.
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return "", errors.New("no editor set")
	}

	f, err := os.CreateTemp("", "frit-prompt-*.md")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString(prompt); err != nil {
		_ = f.Close()

		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	args := append(fields[1:], f.Name())
	cmd := exec.Command(fields[0], args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor: %w", err)
	}

	edited, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}

	return string(edited), nil
}

// withNote folds a rider into the composed prompt as its own paragraph.
// The subject stays the tool's — a slash command naming a plan and a
// phase — and the note rides beneath it. An empty note changes nothing.
func withNote(prompt, note string) string {
	if note == "" {
		return prompt
	}

	return prompt + "\n\n" + note
}

// renderStart prints the escalation as a table or emits it as JSON.
func renderStart(c *cli, rt *runtime, doc *report.StartDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printStart(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printStart writes the escalation: the claim, worktree, agent, prompt
// and focus start ran or would run, or the reason it was refused. The
// whole plan is shown either way, because seeing the escalation is the
// point — under a dry run before it happens, as a recipe the reader is
// invited to run again with --go; under --go as a handoff already
// running in another pane, so the prompt is labelled running: and closes
// with the directive not to run it here.
func printStart(out io.Writer, doc *report.StartDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		if doc.Warning != "" {
			_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
		}
		return
	}

	head := "start plan %d — %s  (dry run)\n"
	switch {
	case doc.Started && doc.Resumed:
		head = "resumed plan %d — %s\n"
	case doc.Started:
		head = "started plan %d — %s\n"
	}
	_, _ = fmt.Fprintf(out, head, doc.Plan.ID, doc.Plan.Title)
	_, _ = fmt.Fprintf(out, "  claim:    %s  (base %s)\n", doc.Branch, doc.Base)
	_, _ = fmt.Fprintf(out, "  worktree: %s\n", doc.Lane)
	_, _ = fmt.Fprintf(out, "  agent:    %s --model %s\n",
		doc.Kind, modelLabel(doc.Tier))
	if doc.Started {
		_, _ = fmt.Fprintf(out, "  running:  %s\n", doc.Prompt)
		_, _ = fmt.Fprintf(out, "  focus:    %s\n", doc.Pane)
		_, _ = fmt.Fprintf(out,
			"plan %d is running in %s; do not run it here — "+
				"watch with %s or move on with frit board\n",
			doc.Plan.ID, doc.Pane, doc.NextAction)
		return
	}
	_, _ = fmt.Fprintf(out, "  prompt:   %s\n", doc.Prompt)
	_, _ = fmt.Fprintln(out, "  focus:    the new pane")
	_, _ = fmt.Fprintln(out, "run again with --go to execute")
}
