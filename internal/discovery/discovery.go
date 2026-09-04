// Package discovery answers the questions asked before dispatching:
// what can I start, what next, where is that plan, what blocks this.
//
// It is pure. Every function here works over an in-memory slice of
// plans that a caller has already gathered from git, so the DAG walk,
// the readiness rule and the selector are all testable against a
// hand-built fleet with no repository on disk. Gathering that slice is
// the caller's job; deciding what it means is this package's.
package discovery

import (
	"strings"
	"time"

	"github.com/jeduden/frit/internal/planmeta"
)

// Plan is one plan as discovery sees it: its identity, its lifecycle,
// the plans it waits on, and whether a lane already holds it.
//
// It is a flattened view of the fleet index — the authoritative version
// of each plan, keyed across host, repo and id — carrying only what the
// discovery verbs read.
type Plan struct {
	// Key is the canonical host:repo:id identity across the fleet.
	Key string
	// Repo is the repository the plan lives in. Dependencies resolve
	// within a repository, since a plan id is only unique there.
	Repo string
	// ID is the plan id, unique within its repository.
	ID int64
	// Status is the four-value lifecycle emoji.
	Status string
	// Title and Summary are the human description, and what find reads.
	Title   string
	Summary string
	// Model is the tier the plan asks for.
	Model string
	// Goal is the prose of the plan's `## Goal` section, folded to one
	// line. Empty when the plan carries no such section.
	Goal string
	// DependsOn is the plan ids this plan waits on, within its repo.
	DependsOn []int64
	// Phases is the phase ledger, if the plan carries one.
	Phases []planmeta.Phase
	// Path is the repository-relative plan file path.
	Path string
	// Branches are the short branch names that carry or claim the plan,
	// matched by the slug form of a selector.
	Branches []string
	// Held reports whether a lane currently claims this plan.
	Held bool
	// Holds are the branch names that claim this plan, the lanes a
	// holder works it on. Empty when nobody holds it.
	Holds []string
	// HoldTip is the commit the plan's id-only work ref points at, ""
	// when no lease ref exists. It is what the staleness observer
	// watches and exactly what a takeover CASes on.
	HoldTip string
	// Stale reports a held plan whose takeover window has matured: the
	// tip sat unchanged for more than the window under sound sampling,
	// so the hold is a takeover candidate.
	Stale bool
	// StaleFor is how long the tip has been observed unchanged.
	StaleFor time.Duration
	// Voided carries the observer's reason the window was thrown away
	// and restarted — a sample gap wider than the bound — "" when the
	// current window was never voided. It lets a not-matured refusal
	// explain a span that keeps resetting instead of just naming a
	// short StaleFor.
	Voided string
	// Dead reports a held plan whose bound session herdr positively
	// confirms is gone — a takeover candidate at once, with no
	// staleness window consulted at all (S76). Distinct from Stale:
	// a live session that simply has not renewed yet is not Dead, and
	// an unreachable herdr answers Dead false, falling back to Stale.
	Dead bool
}

// NotStarted reports a plan nobody has begun.
func (p Plan) NotStarted() bool {
	return p.Status == planmeta.StatusNotStarted
}

// Done reports a completed plan.
func (p Plan) Done() bool {
	return p.Status == planmeta.StatusDone
}

// Superseded reports a plan replaced by another. Like a done plan, it
// is never waiting on its dependencies, so it counts as no downstream
// work when ranking what to start.
func (p Plan) Superseded() bool {
	return p.Status == planmeta.StatusSuperseded
}

// InProgress reports a plan currently being worked.
func (p Plan) InProgress() bool {
	return p.Status == planmeta.StatusInProgress
}

// Unfinished reports a plan that is still outstanding — begun or not,
// but neither done nor superseded. It is the board's default subject:
// the work that remains.
func (p Plan) Unfinished() bool {
	return p.NotStarted() || p.InProgress()
}

// Deserted reports a held plan whose bound session herdr confirms
// gone while its takeover window has yet to mature — the lane nobody
// can resume from its own token, and the one a live pane on the branch
// turns from "nobody is here" into "someone is". A matured window is
// the stale reading's own cell and is not deserted. Every site that
// acts on the deserted reading — start's refusals, the orphans walk,
// the survey's ask — shares this one spelling of it.
func (p Plan) Deserted() bool {
	return p.Held && p.Dead && !p.Stale
}

// NextPhase returns the first phase not at ✅ — the phase frit next
// points at, and the one an executor defaults to. A plan with no ledger
// or with every phase done has no next phase and reports false.
func (p Plan) NextPhase() (planmeta.Phase, bool) {
	return planmeta.Plan{Phases: p.Phases}.FirstOpenPhase()
}

// matchesSlug reports whether a lowered fragment appears in the plan's
// title, its path, or any branch that carries it — the identifiers a
// person remembers a plan by when they have forgotten its id.
func (p Plan) matchesSlug(fragment string) bool {
	if strings.Contains(strings.ToLower(p.Title), fragment) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Path), fragment) {
		return true
	}
	for _, b := range p.Branches {
		if strings.Contains(strings.ToLower(b), fragment) {
			return true
		}
	}

	return false
}
