package report

import (
	"fmt"

	"github.com/jeduden/frit/internal/herdr"
)

// WholePlanPhase is what a dispatch doc reports in Phase for a plan
// that carries no ledger: the whole plan is the dispatch, so an empty
// phase renders as this label rather than a blank cell that would
// otherwise read as a missing field.
const WholePlanPhase = "whole plan"

// The dispatch documents are what the ladder's mutating verbs report.
// Unlike the read-only board, these commands act — they focus a pane,
// send a composed prompt, or stand a lane up — so their documents say
// what was done, or under a dry run what would be, and never carry an
// agent's reply back.

// DispatchPlan names the plan a dispatch verb resolved: enough to
// confirm the right lane was targeted without re-reading the index.
type DispatchPlan struct {
	Repo  string `json:"repo"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// HoldKind classifies why open found no live pane for a held plan, so
// its next-step projection never recommends a rung that would refuse
// (#122). It is read only for a plan that reads held; a plan with no
// hold at all carries the zero value HoldNone, the same as one whose
// kind was never evaluated.
type HoldKind string

const (
	// HoldNone is the ordinary case: no hold to speak of, or a hold
	// whose kind open never read. The zero value, so a document that
	// never sets it reads as carrying no kind.
	HoldNone HoldKind = ""
	// HoldResumable is a hold whose persisted token proves this
	// machine's own lease, with no agent attending it — the case #122
	// exists to unstick: NextAction resumes it with the very verb that
	// used to refuse.
	HoldResumable HoldKind = "resumable"
	// HoldUnproven is a hold this machine cannot prove as its own — no
	// readable marker, no matching token, or no repository coordinate to
	// read either from — so naming frit start would still name a
	// refusal.
	HoldUnproven HoldKind = "unproven"
	// HoldLive is a hold a live agent is already attending.
	HoldLive HoldKind = "live"
	// HoldUnparked is a hold whose persisted token proves this
	// machine's own unattended lease, but whose local lane carries
	// commits past that token it never pushed — S77's park-first guard
	// would still refuse a resume until `frit yield <id>` parks that
	// suffix, so naming `frit start <id>` here would name a refusal
	// just as surely as HoldUnproven or HoldLive would.
	HoldUnparked HoldKind = "unparked"
)

// OpenDoc is what `frit open` did: the plan it resolved and the pane it
// raised, or the plain fact that no lane was live to raise.
//
// open is rung one, the read-only handoff. It sends no text and starts
// no agent, so the document carries a focus and nothing more.
type OpenDoc struct {
	header
	Root    string       `json:"root"`
	Plan    DispatchPlan `json:"plan"`
	Focused bool         `json:"focused"`
	Target  string       `json:"target"`
	Agent   string       `json:"agent"`
	Status  string       `json:"status"`
	Branch  string       `json:"branch"`
	// HoldKind is the true kind of a held lane open found no live pane
	// for — see HoldKind's own doc. Empty when the plan carries no
	// hold, a lane was focused, or its kind was never read.
	HoldKind HoldKind `json:"hold_kind"`
	// NextAction is the real next step when open raised nothing: a
	// resume, a wait for the takeover window, the fact a live agent
	// already attends, or — for a plan with no hold at all — frit
	// start <id>, the rung that creates a lane, since nudge would
	// refuse a laneless plan. It is empty once a lane is focused (watch
	// it, do not escalate) and empty when presence could not be read,
	// because a lane may run behind the socket. A carried repo-read
	// problem does not touch it — the target plan's presence was still
	// read. The value is not written directly: it is openNextAction of
	// Focused, presenceUnknown and HoldKind, refreshed whenever any of
	// the three changes, so it cannot disagree with them.
	NextAction string `json:"next_action"`
	// presenceUnknown records that open could not read live presence — a
	// herdr it could not reach, or a host it could not query. It stays
	// off the wire (NextAction is the field a consumer reads) but drives
	// the projection, so a focused lane found after an unread host still
	// resolves correctly regardless of call order.
	presenceUnknown bool
	// Problems carries a repository frit could not read and a herdr it
	// could not reach. Presence is the one thing open needs live, but a
	// missing socket, like a broken checkout, is reported, not crashed on.
	Problems []Problem `json:"problems"`
}

// openNextAction derives the real next step open hands a consumer from
// its authoritative facts. A focused lane is watched, not escalated;
// unread presence leaves a lane possible; neither names an escalation.
// Otherwise the projection speaks the kind's own truth: a hold whose
// token proves this machine's own unattended lease resumes with frit
// start <id> — the very verb #122 makes honest for it now — a hold a
// live agent already attends says so rather than naming a send that
// would interrupt it, a hold this machine cannot prove names the wait
// for its takeover window rather than a frit start that would refuse,
// a hold whose local lane carries unparked work names frit yield <id>
// rather than a frit start S77's park-first guard would refuse, and a
// plan with no hold at all (HoldNone) still starts, unchanged.
func openNextAction(focused, presenceUnknown bool, kind HoldKind, id int64) string {
	if focused || presenceUnknown {
		return ""
	}

	switch kind {
	case HoldUnproven:
		return fmt.Sprintf(
			"wait for the takeover window, or take it over once it "+
				"matures with frit start %d", id)
	case HoldLive:
		return "a live agent is already on this lane"
	case HoldUnparked:
		return fmt.Sprintf(
			"run frit yield %d to park its unpushed work first", id)
	default:
		return fmt.Sprintf("frit start %d", id)
	}
}

// NewOpen opens a handoff report for a resolved plan. NextAction is
// projected from the empty starting state: no lane found, presence read,
// so frit start <id>. Focus and PresenceUnknown refresh it as they
// change the facts it derives from.
func NewOpen(root string, repo string, id int64, title string) *OpenDoc {
	d := &OpenDoc{
		header:   newHeader("open"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Problems: []Problem{},
	}
	d.refreshNextAction()

	return d
}

// refreshNextAction reprojects NextAction from the current facts. Every
// method that changes Focused, presenceUnknown or HoldKind calls it,
// which is the one place NextAction is written — it can never lag the
// facts.
func (d *OpenDoc) refreshNextAction() {
	d.NextAction = openNextAction(
		d.Focused, d.presenceUnknown, d.HoldKind, d.Plan.ID)
}

// Focus records the pane open raised and the lane it belongs to. A lane
// to watch is not one to escalate, so the refreshed projection clears
// NextAction.
func (d *OpenDoc) Focus(lane herdr.Lane) {
	d.Focused = true
	d.Target = lane.Pane.PaneID
	d.Agent = lane.Pane.Agent
	d.Status = lane.Pane.Presence()
	d.Branch = lane.Branch
	d.refreshNextAction()
}

// SetHoldKind records the true kind of a held lane open found no live
// pane for, read from outside it off the hold's own marker (#122). The
// refreshed projection speaks it in NextAction.
func (d *OpenDoc) SetHoldKind(kind HoldKind) {
	d.HoldKind = kind
	d.refreshNextAction()
}

// PresenceUnknown records that open could not read live presence — a
// herdr it could not reach, or a host it could not query. A lane may be
// running behind the gap, so the refreshed projection names no
// escalation. This is distinct from AddProblem, which carries a repo
// frit could not read without implying the target's presence went unread.
func (d *OpenDoc) PresenceUnknown() {
	d.presenceUnknown = true
	d.refreshNextAction()
}

// AddProblem records a repository or socket frit could not read. It does
// not touch the projection: a carried repo-read failure does not mean the
// target plan's presence went unread — PresenceUnknown says that.
func (d *OpenDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// NudgeDoc is what `frit nudge` composed and, with --go, sent: the typed
// slash command it derived from the plan, the tier the plan declares,
// and the idle lane it targeted.
//
// Rung two is dry-run by default: the document reports the composition
// whether or not it was sent, so a person or an agent sees exactly what
// would go before it goes. It never carries a reply — nudge sends and
// hands over.
type NudgeDoc struct {
	header
	Root string       `json:"root"`
	Plan DispatchPlan `json:"plan"`
	// Phase is the phase the command names; Prompt is the whole composed
	// slash command; Tier is the model the plan declares for it, shown
	// but never sent — nudge prompts an agent already at its tier.
	Phase  string `json:"phase"`
	Prompt string `json:"prompt"`
	Tier   string `json:"tier"`
	// Target is the pane a send would land in, empty when no lane is live
	// to take it.
	Target string `json:"target"`
	// Go is whether --go was given; Sent is whether text actually went.
	// They differ on a refusal: --go with a busy lane sends nothing.
	Go   bool `json:"go"`
	Sent bool `json:"sent"`
	// Refused is why a send was withheld — a busy lane, or none live —
	// empty when nudge was free to send.
	Refused  string    `json:"refused"`
	Problems []Problem `json:"problems"`
}

// NewNudge opens a nudge report for a resolved plan and its composition.
func NewNudge(
	root string, repo string, id int64, title,
	phase, tier, prompt string, wantGo bool,
) *NudgeDoc {
	if phase == "" {
		phase = WholePlanPhase
	}

	return &NudgeDoc{
		header:   newHeader("nudge"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Phase:    phase,
		Prompt:   prompt,
		Tier:     tier,
		Go:       wantGo,
		Problems: []Problem{},
	}
}

// SetTarget records the pane a send would land in.
func (d *NudgeDoc) SetTarget(pane string) { d.Target = pane }

// Refuse records why a send was withheld, leaving Sent false.
func (d *NudgeDoc) Refuse(reason string) { d.Refused = reason }

// MarkSent records that the composed prompt went to the target.
func (d *NudgeDoc) MarkSent() { d.Sent = true }

// AddProblem records a socket frit could not read.
func (d *NudgeDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// ClaimDoc is what `frit claim` did: the hold branch it minted for a
// plan, or the reason the plan was not claimable.
//
// The claim is frit's own atomic lease — an empty marker commit pushed
// with --force-with-lease. The document reports the branch it wrote and
// the base it dated the lease against, or, when the plan was already
// held, blocked or lost to another machine, why nothing was minted.
type ClaimDoc struct {
	header
	Root string       `json:"root"`
	Plan DispatchPlan `json:"plan"`
	// Branch is the hold branch the claim mints, reported even on a
	// refusal so a reader sees the name that would have been written.
	Branch string `json:"branch"`
	// Base is the sha the lease was dated against, set only when minted.
	Base string `json:"base"`
	// Claimed is whether the lease was minted; Refused is why it was not,
	// empty when it was. A plan already held, blocked by an unfinished
	// dependency, or lost to another machine is refused, not leased.
	Claimed bool   `json:"claimed"`
	Refused string `json:"refused"`
	// Resumed reports whether the lane resumed its own lease by its
	// persisted token rather than acquiring or taking one over — the
	// self-resume path, which consults no staleness window at all
	// (F9, F11, S3, S21). It never appears alongside a refusal.
	Resumed bool `json:"resumed"`
	// Worktree is the isolated checkout the claim stood up for the lane,
	// so an agent works it there rather than in the shared clone. Empty
	// when the lease was refused, or when standing the worktree up failed.
	Worktree string `json:"worktree"`
	// Warning is a non-fatal failure that left the atomic lease standing —
	// herdr could not stand the worktree up. The ref is minted first, so a
	// failed checkout is a warning, not a lost claim. Empty when none.
	Warning string `json:"warning"`
	// Scavenged is the work ref a refusal cleaned up on landed evidence,
	// "" when nothing was scavenged; Rescue is where its unlanded work was
	// parked, "" when the chain held nothing a delete could destroy.
	Scavenged string    `json:"scavenged"`
	Rescue    string    `json:"rescue"`
	Problems  []Problem `json:"problems"`
}

// NewClaim opens a claim report for a resolved plan and the branch it
// would mint.
func NewClaim(
	root string, repo string, id int64, title, branch string,
) *ClaimDoc {
	return &ClaimDoc{
		header:   newHeader("claim"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Branch:   branch,
		Problems: []Problem{},
	}
}

// Minted records a lease that went through, dated against a base commit.
func (d *ClaimDoc) Minted(baseSHA string) {
	d.Claimed = true
	d.Base = baseSHA
}

// MarkResumed records a lease resumed by its own lane, on the strength
// of its persisted token rather than a fresh acquire or a takeover.
func (d *ClaimDoc) MarkResumed() {
	d.Claimed = true
	d.Resumed = true
}

// Stood records the isolated worktree the claim stood up for the lane.
func (d *ClaimDoc) Stood(path string) { d.Worktree = path }

// Warn records a non-fatal failure that left the lease standing — herdr
// could not stand the worktree up behind a minted claim.
func (d *ClaimDoc) Warn(reason string) { d.Warning = reason }

// Refuse records why no lease was minted, leaving Claimed false.
func (d *ClaimDoc) Refuse(reason string) { d.Refused = reason }

// Unwound records a claim whose fresh lease was released after its
// worktree stand-up failed: no lane ever persisted a token for it, so
// the atomic hold is undone rather than left standing with nothing
// behind it. Claimed and Base, set by the earlier Minted, are both
// cleared, so the report reads exactly like an ordinary refusal
// rather than a claim that half-succeeded.
func (d *ClaimDoc) Unwound(reason string) {
	d.Claimed = false
	d.Base = ""
	d.Refused = reason
}

// ScavengedRef records the work ref a refusal cleaned up on landed
// evidence, and where its unlanded work was parked, if anywhere.
func (d *ClaimDoc) ScavengedRef(branch, rescue string) {
	d.Scavenged = branch
	d.Rescue = rescue
}

// AddProblem records a repository frit could not read.
func (d *ClaimDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// YieldDoc is what `frit yield` did: a fenced lane's local divergence
// parked to a rescue ref and its worktree torn down through herdr, or
// the reason nothing was — the lane still holds the live lease, and
// yield is for the fenced, not an alias for release.
type YieldDoc struct {
	header
	Root   string       `json:"root"`
	Plan   DispatchPlan `json:"plan"`
	Branch string       `json:"branch"`
	// Rescue is the ref local divergence was parked to, "" when yield
	// was refused.
	Rescue string `json:"rescue"`
	// TornDown is whether herdr tore the lane's own worktree down.
	TornDown bool `json:"torn_down"`
	// Refused is why nothing was parked or torn down, empty when yield
	// proceeded.
	Refused string `json:"refused"`
	// Warning is a non-fatal failure that left the parked rescue
	// standing — herdr could not be read, or could not tear the lane
	// down. Empty when none.
	Warning  string    `json:"warning"`
	Problems []Problem `json:"problems"`
}

// NewYield opens a yield report for a resolved plan and the branch its
// work ref carries.
func NewYield(root, repo string, id int64, title, branch string) *YieldDoc {
	return &YieldDoc{
		header:   newHeader("yield"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Branch:   branch,
		Problems: []Problem{},
	}
}

// Parked records the rescue ref local divergence was pushed to.
func (d *YieldDoc) Parked(rescue string) { d.Rescue = rescue }

// Torn records that herdr tore the lane's own worktree down.
func (d *YieldDoc) Torn() { d.TornDown = true }

// Refuse records why nothing was parked or torn down.
func (d *YieldDoc) Refuse(reason string) { d.Refused = reason }

// Warn records a non-fatal failure that left the parked rescue
// standing.
func (d *YieldDoc) Warn(reason string) { d.Warning = reason }

// AddProblem records a repository frit could not read.
func (d *YieldDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// ReleaseDoc is what `frit release` did: the calling lane's own lease
// marked released, or the reason nothing changed.
//
// A plan nobody holds, or one whose hold already reads as released, is
// a no-op — there was nothing left to end, not a door release found
// shut. A plan held live or matured by another lane is refused: only
// that lane's own persisted token can release it; a stranger takes a
// matured one over through claim instead. A hold whose work already
// landed is scavenged rather than released.
type ReleaseDoc struct {
	header
	Root   string       `json:"root"`
	Plan   DispatchPlan `json:"plan"`
	Branch string       `json:"branch"`
	// Released is whether a release marker was pushed.
	Released bool `json:"released"`
	// Refused is why a foreign hold was left standing, empty when
	// release proceeded or found nothing to do.
	Refused string `json:"refused"`
	// NoOp says why nothing changed though nothing stood in the way
	// either — the plan was already free, or its hold already reads
	// as released. Empty when release refused, released or scavenged.
	NoOp string `json:"no_op"`
	// Scavenged is the work ref release cleaned up on landed evidence,
	// "" when nothing was scavenged; Rescue is where its unlanded work
	// was parked, "" when the chain held nothing a delete could
	// destroy.
	Scavenged string `json:"scavenged"`
	Rescue    string `json:"rescue"`
	// Warning is a non-fatal failure alongside a scavenge — the ref
	// itself is still gone, or still standing, either way reported
	// rather than silently swallowed.
	Warning  string    `json:"warning"`
	Problems []Problem `json:"problems"`
}

// NewRelease opens a release report for a resolved plan and the branch
// its work ref carries.
func NewRelease(root, repo string, id int64, title, branch string) *ReleaseDoc {
	return &ReleaseDoc{
		header:   newHeader("release"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Branch:   branch,
		Problems: []Problem{},
	}
}

// MarkReleased records that a release marker was pushed.
func (d *ReleaseDoc) MarkReleased() { d.Released = true }

// Refuse records why a foreign hold was left standing.
func (d *ReleaseDoc) Refuse(reason string) { d.Refused = reason }

// Nothing records that release found nothing to do, and why.
func (d *ReleaseDoc) Nothing(reason string) { d.NoOp = reason }

// ScavengedRef records the work ref release cleaned up on landed
// evidence, and where its unlanded work was parked, if anywhere.
func (d *ReleaseDoc) ScavengedRef(branch, rescue string) {
	d.Scavenged = branch
	d.Rescue = rescue
}

// Warn records a non-fatal failure alongside a scavenge.
func (d *ReleaseDoc) Warn(reason string) { d.Warning = reason }

// AddProblem records a repository frit could not read.
func (d *ReleaseDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// The handoff values a StartDoc reports: the one axis a consumer keys on
// instead of re-deriving "the prompt is not mine" from started/refused.
const (
	// HandoffPreview is a dry run the caller would cause with --go.
	HandoffPreview = "preview"
	// HandoffRunning is the prompt dispatched to a spawned agent now
	// executing it.
	HandoffRunning = "running"
	// HandoffNone is a refused escalation: nothing runs.
	HandoffNone = "none"
)

// startNextAction derives the verb a consumer runs instead of the
// dispatched prompt from the handoff alone. A running handoff hands over
// frit open <id>, a look at the lane; every other handoff leaves the
// prompt as the recipe and names nothing.
func startNextAction(handoff string, id int64) string {
	if handoff == HandoffRunning {
		return fmt.Sprintf("frit open %d", id)
	}

	return ""
}

// StartDoc is the full escalation `frit start` composes: the claim it
// would mint, the worktree and agent herdr would stand up, the tier the
// plan declares, and the typed prompt it would send.
//
// Rung three is dry-run by default like the rungs below it, and its
// document reports the whole plan whether or not it ran, so the escalation
// stays auditable before anything is spawned. Every mutation is delegated
// — frit mints the claim and hands the worktree, the agent and the pane
// to herdr — and no reply is ever read back.
type StartDoc struct {
	header
	Root string       `json:"root"`
	Plan DispatchPlan `json:"plan"`
	// Phase and Tier come from the plan; Kind is the agent herdr starts.
	Phase string `json:"phase"`
	Tier  string `json:"tier"`
	Kind  string `json:"kind"`
	// Branch is the hold the claim mints; Base is the ref it is dated
	// against; Lane is the worktree path herdr would check it out into.
	Branch string `json:"branch"`
	Base   string `json:"base"`
	Lane   string `json:"lane"`
	// Prompt is the whole composed slash command, the note folded in.
	// See PromptDispatched for whether it has already been sent into a
	// pane rather than left for the caller to run.
	Prompt string `json:"prompt"`
	// PromptDispatched is true exactly when Handoff is HandoffRunning —
	// the prompt was sent into the pane — and false for a preview or a
	// refusal, where Prompt is still the caller's recipe. It is written
	// only through setHandoff, beside NextAction, so the three cannot
	// part.
	PromptDispatched bool `json:"prompt_dispatched"`
	// Go is whether --go was given; Started is whether the escalation ran.
	// They differ on a refusal: --go on an unstartable plan runs nothing.
	Go      bool `json:"go"`
	Started bool `json:"started"`
	// Resumed reports whether the escalation ran on the lane's own
	// lease, resumed by its persisted token rather than acquired or
	// taken over — the self-resume path (F9, F11, S3, S21). It never
	// appears alongside a refusal.
	Resumed bool `json:"resumed"`
	// Pane is the pane herdr opened and the agent runs in, set only once
	// the escalation has run.
	Pane string `json:"pane"`
	// Handoff is the one axis a consumer keys on instead of re-deriving
	// "the prompt is not mine" from started/refused. It is one of
	// HandoffPreview, HandoffRunning or HandoffNone, and is written only
	// through setHandoff, which reprojects NextAction and
	// PromptDispatched alongside it.
	Handoff string `json:"handoff"`
	// NextAction is the verb a consumer runs instead of the dispatched
	// Prompt: frit open <id> once Handoff is HandoffRunning, empty on a
	// preview or a refusal, where Prompt is still the recipe to run. It is
	// not written directly — it is startNextAction of Handoff, refreshed
	// by setHandoff, so it cannot disagree with the handoff it mirrors.
	NextAction string `json:"next_action"`
	// Refused is why the escalation was withheld — a plan not startable,
	// or a claim lost to another machine — empty when start proceeded.
	Refused string `json:"refused"`
	// Scavenged is the work ref a refusal cleaned up on landed
	// evidence, "" when nothing was scavenged; Rescue is where its
	// unlanded work was parked, "" when the chain held nothing a
	// delete could destroy.
	Scavenged string `json:"scavenged"`
	Rescue    string `json:"rescue"`
	// Warning is a non-fatal failure alongside a scavenge.
	Warning  string    `json:"warning"`
	Problems []Problem `json:"problems"`
}

// StartPlan carries the composed escalation into a StartDoc, so the
// constructor stays one call rather than a long positional list.
type StartPlan struct {
	Phase, Tier, Kind, Branch, Base, Lane, Prompt string
}

// NewStart opens an escalation report for a resolved plan and the
// composition start would run.
func NewStart(
	root string, repo string, id int64, title string,
	sp StartPlan, wantGo bool,
) *StartDoc {
	phase := sp.Phase
	if phase == "" {
		phase = WholePlanPhase
	}

	d := &StartDoc{
		header:   newHeader("start"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Phase:    phase,
		Tier:     sp.Tier,
		Kind:     sp.Kind,
		Branch:   sp.Branch,
		Base:     sp.Base,
		Lane:     sp.Lane,
		Prompt:   sp.Prompt,
		Go:       wantGo,
		Problems: []Problem{},
	}
	d.setHandoff(HandoffPreview)

	return d
}

// setHandoff moves the handoff and reprojects NextAction and
// PromptDispatched from it in the same step, so the three stay in sync
// and cannot part. It is the one writer of all three fields.
func (d *StartDoc) setHandoff(handoff string) {
	d.Handoff = handoff
	d.NextAction = startNextAction(handoff, d.Plan.ID)
	d.PromptDispatched = handoff == HandoffRunning
}

// Refuse records why the escalation was withheld, leaving Started false.
func (d *StartDoc) Refuse(reason string) {
	d.Refused = reason
	d.setHandoff(HandoffNone)
}

// MarkStarted records that the escalation ran and the pane it stood up.
func (d *StartDoc) MarkStarted(pane string) {
	d.Started = true
	d.Pane = pane
	d.setHandoff(HandoffRunning)
}

// MarkResumed records that the escalation is standing the lane back up
// on a lease resumed by its own persisted token, rather than one
// acquired or taken over.
func (d *StartDoc) MarkResumed() { d.Resumed = true }

// ScavengedRef records the work ref a refusal cleaned up on landed
// evidence, and where its unlanded work was parked, if anywhere.
func (d *StartDoc) ScavengedRef(branch, rescue string) {
	d.Scavenged = branch
	d.Rescue = rescue
}

// Warn records a non-fatal failure alongside a scavenge.
func (d *StartDoc) Warn(reason string) { d.Warning = reason }

// AddProblem records a repository frit could not read.
func (d *StartDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
