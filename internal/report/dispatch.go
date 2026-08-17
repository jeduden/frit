package report

import "github.com/jeduden/frit/internal/herdr"

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
	// Problems carries a repository frit could not read and a herdr it
	// could not reach. Presence is the one thing open needs live, but a
	// missing socket, like a broken checkout, is reported, not crashed on.
	Problems []Problem `json:"problems"`
}

// NewOpen opens a handoff report for a resolved plan.
func NewOpen(root string, repo string, id int64, title string) *OpenDoc {
	return &OpenDoc{
		header:   newHeader("open"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Problems: []Problem{},
	}
}

// Focus records the pane open raised and the lane it belongs to.
func (d *OpenDoc) Focus(lane herdr.Lane) {
	d.Focused = true
	d.Target = lane.Pane.PaneID
	d.Agent = lane.Pane.Agent
	d.Status = lane.Pane.Presence()
	d.Branch = lane.Branch
}

// AddProblem records a socket frit could not read.
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
	Claimed  bool      `json:"claimed"`
	Refused  string    `json:"refused"`
	Problems []Problem `json:"problems"`
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

// Refuse records why no lease was minted, leaving Claimed false.
func (d *ClaimDoc) Refuse(reason string) { d.Refused = reason }

// AddProblem records a repository frit could not read.
func (d *ClaimDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
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
	Prompt string `json:"prompt"`
	// Go is whether --go was given; Started is whether the escalation ran.
	// They differ on a refusal: --go on an unstartable plan runs nothing.
	Go      bool `json:"go"`
	Started bool `json:"started"`
	// Pane is the pane herdr opened and the agent runs in, set only once
	// the escalation has run.
	Pane string `json:"pane"`
	// Refused is why the escalation was withheld — a plan not startable,
	// or a claim lost to another machine — empty when start proceeded.
	Refused  string    `json:"refused"`
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
	return &StartDoc{
		header:   newHeader("start"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Phase:    sp.Phase,
		Tier:     sp.Tier,
		Kind:     sp.Kind,
		Branch:   sp.Branch,
		Base:     sp.Base,
		Lane:     sp.Lane,
		Prompt:   sp.Prompt,
		Go:       wantGo,
		Problems: []Problem{},
	}
}

// Refuse records why the escalation was withheld, leaving Started false.
func (d *StartDoc) Refuse(reason string) { d.Refused = reason }

// MarkStarted records that the escalation ran and the pane it stood up.
func (d *StartDoc) MarkStarted(pane string) {
	d.Started = true
	d.Pane = pane
}

// AddProblem records a repository frit could not read.
func (d *StartDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
